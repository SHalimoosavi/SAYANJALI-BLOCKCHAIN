"""Tests for blockchain/network/ratelimit.py."""

from __future__ import annotations

import time

from blockchain.network.ratelimit import PeerRateLimiter


def test_allows_requests_within_limit():
    limiter = PeerRateLimiter(max_requests=3, window_seconds=60)
    assert limiter.allow("peer-a")
    assert limiter.allow("peer-a")
    assert limiter.allow("peer-a")


def test_rejects_requests_over_limit():
    limiter = PeerRateLimiter(max_requests=2, window_seconds=60)
    assert limiter.allow("peer-a")
    assert limiter.allow("peer-a")
    assert not limiter.allow("peer-a")


def test_limits_are_independent_per_key():
    limiter = PeerRateLimiter(max_requests=1, window_seconds=60)
    assert limiter.allow("peer-a")
    assert limiter.allow("peer-b")
    assert not limiter.allow("peer-a")
    assert not limiter.allow("peer-b")


def test_window_expires_old_requests():
    limiter = PeerRateLimiter(max_requests=1, window_seconds=0.05)
    assert limiter.allow("peer-a")
    assert not limiter.allow("peer-a")
    time.sleep(0.1)
    assert limiter.allow("peer-a")


def test_reset_clears_history():
    limiter = PeerRateLimiter(max_requests=1, window_seconds=60)
    limiter.allow("peer-a")
    assert not limiter.allow("peer-a")
    limiter.reset()
    assert limiter.allow("peer-a")
