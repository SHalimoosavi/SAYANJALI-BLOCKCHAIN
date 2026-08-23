"""
Basic per-peer rate limiting for SAYANJALI BLOCKCHAIN's P2P endpoints.

This is a deliberately simple, in-memory, fixed-window limiter -- enough
to blunt an obviously abusive peer hammering an endpoint, not a
production-grade defense. It is process-local: it resets on process
restart and does not coordinate across a multi-instance deployment. That
is an accepted, documented limitation for this MVP, not an oversight --
building distributed rate limiting is explicitly out of scope.

Phase 6.5 fix: the original implementation keyed request history by an
arbitrary, attacker-controlled string (the claimed `from_peer` address)
in a plain `defaultdict`, which never removed an entry once created --
sending one request per unique fake key grew that dict forever. This
version bounds the number of distinct keys tracked at once via LRU
eviction (the same bounded-cache principle `blockchain.utils.BoundedSet`
already uses elsewhere in this codebase), so total memory use has a hard
ceiling regardless of how many distinct (fake or real) keys an attacker
presents.
"""

from __future__ import annotations

import time
from collections import OrderedDict, deque


class PeerRateLimiter:
    """
    Fixed-window request counter keyed by peer identifier (typically the
    claimed peer address or the connecting client's host), with a bounded
    total number of tracked keys.
    """

    def __init__(
        self, max_requests: int, window_seconds: float, max_keys: int = 10000
    ) -> None:
        self._max_requests = max_requests
        self._window_seconds = window_seconds
        self._max_keys = max_keys
        # OrderedDict used as an LRU: touching a key moves it to the end;
        # the oldest (least recently used) key is evicted first once the
        # tracked-key count exceeds max_keys. This bounds memory even
        # under an attacker sending a unique fake key on every request.
        self._hits: "OrderedDict[str, deque]" = OrderedDict()

    def allow(self, key: str) -> bool:
        """
        Record a request attempt for `key` and return whether it is
        within the allowed rate.
        """
        now = time.time()
        window_start = now - self._window_seconds

        if key in self._hits:
            self._hits.move_to_end(key)
            hits = self._hits[key]
        else:
            hits = deque()
            self._hits[key] = hits
            if len(self._hits) > self._max_keys:
                self._hits.popitem(last=False)  # evict least-recently-used key

        while hits and hits[0] < window_start:
            hits.popleft()

        if len(hits) >= self._max_requests:
            return False

        hits.append(now)
        return True

    def tracked_key_count(self) -> int:
        """Return the number of distinct keys currently tracked. For tests/introspection."""
        return len(self._hits)

    def reset(self) -> None:
        """Clear all tracked request history. Primarily useful for tests."""
        self._hits.clear()
