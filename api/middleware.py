"""
Request body size limiting middleware for SAYANJALI BLOCKCHAIN's API.

Phase 6.5 fix: the original per-route size check read the
`Content-Length` header *inside* the route function, after FastAPI/
Starlette had already read, buffered, and Pydantic-parsed the entire
request body. That check happened too late to prevent the memory/CPU
cost it was meant to guard against, and was trivially bypassed by a
missing or understated `Content-Length` header (e.g. chunked
transfer-encoding).

This middleware enforces the limit by counting actual bytes as they
arrive over the ASGI `receive` channel, *before* the downstream
application (FastAPI's routing/Pydantic layer) ever sees them. If the
configured limit is exceeded, the request is rejected with a 413
response and the downstream application is never invoked for that
request at all -- no route-level code runs, no Pydantic parsing happens,
regardless of what the client claimed via headers.

This buffers the body up to the configured limit before handing it to
the downstream app (a "buffer-then-replay" pattern): once a message
stream has started flowing to a downstream ASGI app, that app owns
sending the HTTP response, so a middleware cannot safely abort partway
through and independently send its own error response without the two
responses colliding. Buffering first, and only starting the downstream
app once the body is known to be within bounds, avoids that entirely.
Since bodies are rejected once they exceed the configured cap, peak
memory use from this buffering is bounded by that same cap -- it does
not offset the protection this middleware provides.
"""

from __future__ import annotations

from typing import Callable

from starlette.types import ASGIApp, Message, Receive, Scope, Send

from blockchain.utils import get_logger

logger = get_logger("api.middleware")


class MaxBodySizeMiddleware:
    """
    ASGI middleware enforcing a maximum request body size, with optional
    per-path-prefix overrides for endpoints that need a different limit
    than the default (e.g. a small cap for handshake/registration bodies,
    a larger one for block propagation).
    """

    def __init__(
        self,
        app: ASGIApp,
        default_max_bytes: int,
        path_overrides: dict[str, int] | None = None,
    ) -> None:
        self.app = app
        self.default_max_bytes = default_max_bytes
        self.path_overrides = path_overrides or {}

    def _limit_for_path(self, path: str) -> int:
        for prefix, limit in self.path_overrides.items():
            if path.startswith(prefix):
                return limit
        return self.default_max_bytes

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        limit = self._limit_for_path(scope.get("path", ""))

        # Fast rejection when the client honestly declares an oversized
        # body -- avoids even starting to read in the common case of a
        # truthful Content-Length header.
        headers = dict(scope.get("headers", []))
        declared_length = headers.get(b"content-length")
        if declared_length is not None:
            try:
                if int(declared_length) > limit:
                    await self._reject(send, limit)
                    return
            except ValueError:
                pass  # Malformed header; fall through to byte-counted enforcement.

        buffered: list[bytes] = []
        total = 0
        more_body = True

        while more_body:
            message = await receive()
            if message["type"] != "http.request":
                # Non-body message (e.g. disconnect) -- stop buffering and
                # let the downstream app handle it via the replay wrapper.
                buffered.append(message)
                break

            chunk = message.get("body", b"")
            total += len(chunk)
            if total > limit:
                await self._reject(send, limit)
                return

            buffered.append(message)
            more_body = message.get("more_body", False)

        replayed = list(buffered)

        async def replay_receive() -> Message:
            if replayed:
                item = replayed.pop(0)
                return item if isinstance(item, dict) and "type" in item else {
                    "type": "http.request", "body": b"", "more_body": False
                }
            return await receive()

        await self.app(scope, replay_receive, send)

    @staticmethod
    async def _reject(send: Send, limit: int) -> None:
        body = (
            f'{{"detail":"Request body exceeds the {limit}-byte limit for this endpoint."}}'
        ).encode("utf-8")
        await send(
            {
                "type": "http.response.start",
                "status": 413,
                "headers": [(b"content-type", b"application/json")],
            }
        )
        await send({"type": "http.response.body", "body": body})
