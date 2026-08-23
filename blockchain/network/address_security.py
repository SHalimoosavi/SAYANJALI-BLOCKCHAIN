"""
Centralized peer address validation for SAYANJALI BLOCKCHAIN's P2P layer.

Every outbound HTTP request this node makes to a peer -- whether
triggered by registration, propagation, or synchronization -- must have
its target address validated through `validate_peer_address()` in this
module. No other module should implement its own ad-hoc address
validation; that duplication is exactly how one call site ends up
protected while another (like the pre-hardening `/network/sync` endpoint)
does not.

This is a *structural* SSRF mitigation: it rejects malformed URLs,
disallowed schemes, embedded credentials, and -- unless explicitly
configured otherwise -- addresses resolving to loopback, private
(RFC1918), link-local (including cloud metadata endpoints at
169.254.0.0/16), and other non-public IP ranges.

Known limitation, stated plainly: DNS-based validation here is
best-effort, not a complete DNS-rebinding defense. A hostname is resolved
once, at validation time, and checked; nothing pins the connection to
that specific resolved IP at request time, so a sufficiently motivated
attacker controlling DNS could in principle rebind a hostname to a
private address between validation and the actual HTTP request. A full
fix requires resolving once and connecting directly to the pinned IP
(bypassing the HTTP client's own resolution), which is meaningfully more
complex and is left as documented future work rather than silently
overclaimed as solved here.
"""

from __future__ import annotations

import ipaddress
import socket
from urllib.parse import urlparse

VALID_SCHEMES = {"http", "https"}
MAX_ADDRESS_LENGTH = 256


def _is_disallowed_ip(ip: ipaddress.IPv4Address | ipaddress.IPv6Address) -> bool:
    """
    Return True if `ip` falls in a range that should never be an outbound
    P2P target when private-address peering is disabled: loopback,
    private (RFC1918/RFC4193), link-local (including the 169.254.0.0/16
    cloud metadata range), reserved, multicast, or unspecified.
    """
    return (
        ip.is_loopback
        or ip.is_private
        or ip.is_link_local
        or ip.is_reserved
        or ip.is_multicast
        or ip.is_unspecified
    )


def _resolve_hostname_ips(hostname: str) -> list[str]:
    """
    Best-effort resolution of `hostname` to its IP addresses.

    Returns an empty list if resolution fails outright (the caller
    treats an unresolvable hostname as invalid rather than silently
    allowing it through unchecked).
    """
    try:
        infos = socket.getaddrinfo(hostname, None)
        return sorted({info[4][0] for info in infos})
    except socket.gaierror:
        return []


def validate_peer_address(
    address: str, allow_private_addresses: bool = True
) -> tuple[bool, str]:
    """
    Validate a peer address for both structural soundness and, unless
    `allow_private_addresses` is True, SSRF-relevant target safety.

    Args:
        address: The candidate peer address, e.g. "http://host:port".
        allow_private_addresses: When True (the default, matching the
            existing local/LAN/Termux multi-node development workflow),
            loopback and private-range addresses are permitted -- this is
            required for `127.0.0.1`-based multi-node testing to keep
            working. When False, this function additionally rejects any
            address resolving to a non-public IP range, which is the
            setting operators should use for anything resembling a
            production or otherwise adversarial deployment.

    Returns:
        (is_valid, reason) -- reason is empty when valid.
    """
    if not address or not isinstance(address, str):
        return False, "Address is empty or not a string."
    if len(address) > MAX_ADDRESS_LENGTH:
        return False, f"Address exceeds {MAX_ADDRESS_LENGTH} characters."

    try:
        parsed = urlparse(address)
    except ValueError as exc:
        return False, f"Malformed address: {exc}"

    if parsed.scheme.lower() not in VALID_SCHEMES:
        return False, f"Unsupported scheme: {parsed.scheme!r}. Only http/https allowed."

    if parsed.username is not None or parsed.password is not None:
        return False, "Credentials embedded in the address are not allowed."

    if not parsed.hostname:
        return False, "Address has no hostname."

    if parsed.path not in ("", "/"):
        return False, "Address must not include a path."

    if parsed.query or parsed.fragment:
        return False, "Address must not include a query string or fragment."

    if parsed.port is not None and not (1 <= parsed.port <= 65535):
        return False, f"Invalid port: {parsed.port}."

    if allow_private_addresses:
        return True, ""

    hostname = parsed.hostname
    try:
        ip = ipaddress.ip_address(hostname)
        candidate_ips = [ip]
    except ValueError:
        # Not a literal IP -- treat it as a DNS name and resolve it.
        resolved = _resolve_hostname_ips(hostname)
        if not resolved:
            return False, f"Could not resolve hostname: {hostname!r}."
        try:
            candidate_ips = [ipaddress.ip_address(addr) for addr in resolved]
        except ValueError:
            return False, f"Resolved address for {hostname!r} was not a valid IP."

    for candidate_ip in candidate_ips:
        if _is_disallowed_ip(candidate_ip):
            return False, (
                f"Address resolves to a non-public IP range ({candidate_ip}), "
                "which is disallowed when allow_private_peer_addresses is False."
            )

    return True, ""
