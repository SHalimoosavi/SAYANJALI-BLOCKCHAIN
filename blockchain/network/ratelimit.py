"""
Basic per-peer rate limiting for SAYANJALI BLOCKCHAIN's P2P endpoints.

This is a deliberately simple, in-memory, fixed-window limiter -- enough
to blunt an obviously abusive peer hammering an endpoint, not a
production-grade defense (it resets on process restart, is per-process
rather than shared across a deployment, and does not distinguish
malicious traffic from a legitimate burst). See the Security section of
the README for the full list of known limitations this MVP accepts.
"""

from __future__ import annotations

import time
from collections import defaultdict, deque


class PeerRateLimiter:
    """
    Fixed-window request counter keyed by peer identifier (typically the
    claimed peer address or the connecting client's host).
    """

    def __init__(self, max_requests: int, window_seconds: float) -> None:
        self._max_requests = max_requests
        self._window_seconds = window_seconds
        self._hits: dict[str, deque] = defaultdict(deque)

    def allow(self, key: str) -> bool:
        """
        Record a request attempt for `key` and return whether it is
        within the allowed rate.
        """
        now = time.time()
        window_start = now - self._window_seconds
        hits = self._hits[key]

        while hits and hits[0] < window_start:
            hits.popleft()

        if len(hits) >= self._max_requests:
            return False

        hits.append(now)
        return True

    def reset(self) -> None:
        """Clear all tracked request history. Primarily useful for tests."""
        self._hits.clear()
