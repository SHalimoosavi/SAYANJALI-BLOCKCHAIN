"""Tests for blockchain/network/address_security.py."""

from __future__ import annotations

from blockchain.network.address_security import validate_peer_address


class TestStructuralValidation:
    """Checks that apply regardless of allow_private_addresses."""

    def test_malformed_url_rejected(self):
        ok, _ = validate_peer_address("not-a-url", allow_private_addresses=True)
        assert not ok

    def test_unsupported_scheme_rejected(self):
        ok, reason = validate_peer_address("ftp://example.com", allow_private_addresses=True)
        assert not ok
        assert "scheme" in reason.lower()

    def test_file_scheme_rejected(self):
        ok, _ = validate_peer_address("file:///etc/passwd", allow_private_addresses=True)
        assert not ok

    def test_embedded_credentials_rejected(self):
        ok, reason = validate_peer_address(
            "http://user:pass@127.0.0.1:8000", allow_private_addresses=True
        )
        assert not ok
        assert "credentials" in reason.lower()

    def test_path_rejected(self):
        ok, _ = validate_peer_address(
            "http://127.0.0.1:8000/admin", allow_private_addresses=True
        )
        assert not ok

    def test_query_string_rejected(self):
        ok, _ = validate_peer_address(
            "http://127.0.0.1:8000?x=1", allow_private_addresses=True
        )
        assert not ok

    def test_empty_address_rejected(self):
        ok, _ = validate_peer_address("", allow_private_addresses=True)
        assert not ok

    def test_oversized_address_rejected(self):
        ok, _ = validate_peer_address(
            "http://" + "a" * 300 + ".com", allow_private_addresses=True
        )
        assert not ok

    def test_no_hostname_rejected(self):
        ok, _ = validate_peer_address("http://:8000", allow_private_addresses=True)
        assert not ok

    def test_valid_https_accepted(self):
        ok, reason = validate_peer_address(
            "https://example.com:8443", allow_private_addresses=True
        )
        assert ok, reason


class TestPrivateAddressAllowed:
    """When allow_private_addresses=True (the local/LAN dev default)."""

    def test_loopback_accepted(self):
        ok, reason = validate_peer_address("http://127.0.0.1:8000", allow_private_addresses=True)
        assert ok, reason

    def test_rfc1918_accepted(self):
        ok, reason = validate_peer_address("http://192.168.1.5:8000", allow_private_addresses=True)
        assert ok, reason

    def test_link_local_accepted(self):
        ok, reason = validate_peer_address(
            "http://169.254.1.1:8000", allow_private_addresses=True
        )
        assert ok, reason


class TestPrivateAddressDisallowed:
    """When allow_private_addresses=False (the hardened/production setting)."""

    def test_loopback_rejected(self):
        ok, reason = validate_peer_address("http://127.0.0.1:8000", allow_private_addresses=False)
        assert not ok
        assert "non-public" in reason.lower()

    def test_localhost_hostname_rejected(self):
        ok, reason = validate_peer_address("http://localhost:8000", allow_private_addresses=False)
        assert not ok

    def test_rfc1918_10_range_rejected(self):
        ok, _ = validate_peer_address("http://10.0.0.5:8000", allow_private_addresses=False)
        assert not ok

    def test_rfc1918_172_range_rejected(self):
        ok, _ = validate_peer_address("http://172.16.0.5:8000", allow_private_addresses=False)
        assert not ok

    def test_rfc1918_192_range_rejected(self):
        ok, _ = validate_peer_address("http://192.168.0.5:8000", allow_private_addresses=False)
        assert not ok

    def test_link_local_metadata_endpoint_rejected(self):
        """The classic cloud metadata SSRF target: 169.254.169.254."""
        ok, reason = validate_peer_address(
            "http://169.254.169.254:80", allow_private_addresses=False
        )
        assert not ok
        assert "non-public" in reason.lower()

    def test_ipv6_loopback_rejected(self):
        ok, _ = validate_peer_address("http://[::1]:8000", allow_private_addresses=False)
        assert not ok

    def test_public_ip_accepted(self):
        ok, reason = validate_peer_address("http://8.8.8.8:8000", allow_private_addresses=False)
        assert ok, reason

    def test_unresolvable_hostname_rejected(self):
        ok, reason = validate_peer_address(
            "http://this-host-does-not-exist.invalid:8000", allow_private_addresses=False
        )
        assert not ok
        assert "resolve" in reason.lower()


class TestNormalizationBypassAttempts:
    """Common tricks used to sneak a disallowed address past naive checks."""

    def test_decimal_ip_notation_still_caught_or_rejected(self):
        # 2130706433 == 127.0.0.1 in decimal notation. Python's ipaddress
        # module will not parse this as a literal IP, so it falls through
        # to hostname resolution, which will fail for a non-DNS string --
        # either way, it must not be silently accepted as a safe address.
        ok, _ = validate_peer_address(
            "http://2130706433:8000", allow_private_addresses=False
        )
        assert not ok

    def test_uppercase_scheme_still_validated(self):
        ok, reason = validate_peer_address("HTTP://127.0.0.1:8000", allow_private_addresses=True)
        assert ok, reason

    def test_trailing_dot_hostname(self):
        # "127.0.0.1." (trailing dot) is a valid absolute DNS form some
        # resolvers accept; ensure it doesn't slip past the private-range
        # check when disallowed.
        ok, _ = validate_peer_address(
            "http://127.0.0.1.:8000", allow_private_addresses=False
        )
        assert not ok
