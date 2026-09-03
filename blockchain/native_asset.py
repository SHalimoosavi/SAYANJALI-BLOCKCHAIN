"""Native SYJ monetary primitives and protocol supply constants."""

from __future__ import annotations

from decimal import Decimal, InvalidOperation, ROUND_DOWN
from typing import Union

SYMBOL = "SYJ"
BASE_UNITS_PER_SYJ = 100_000_000  # 1 SYJ = 10^8 indivisible base units.
MAX_SUPPLY_SYJ = Decimal("720000000")
MAX_SUPPLY_BASE_UNITS = int(MAX_SUPPLY_SYJ * BASE_UNITS_PER_SYJ)

AmountInput = Union[str, int, float, Decimal]


def to_base_units(value: AmountInput) -> int:
    """Convert a human SYJ amount to an exact integer base-unit amount."""
    if isinstance(value, bool):
        raise ValueError("SYJ amount must not be boolean.")
    try:
        decimal_value = Decimal(str(value))
    except (InvalidOperation, ValueError, TypeError) as exc:
        raise ValueError(f"Invalid SYJ amount: {value!r}") from exc
    if not decimal_value.is_finite():
        raise ValueError("SYJ amount must be finite.")
    scaled = decimal_value * BASE_UNITS_PER_SYJ
    if scaled != scaled.to_integral_value():
        raise ValueError(
            f"SYJ amount supports at most 8 decimal places: {value!r}."
        )
    return int(scaled)


def from_base_units(base_units: int) -> Decimal:
    """Convert exact integer base units to a Decimal SYJ amount."""
    if isinstance(base_units, bool) or not isinstance(base_units, int):
        raise ValueError("SYJ base units must be an integer.")
    return Decimal(base_units) / Decimal(BASE_UNITS_PER_SYJ)


def format_amount(base_units: int) -> str:
    """Return a canonical fixed-precision human-readable SYJ amount."""
    amount = from_base_units(base_units).quantize(
        Decimal("0.00000001"), rounding=ROUND_DOWN
    )
    return format(amount, "f")


def validate_base_units(value: int, *, allow_zero: bool = False) -> tuple[bool, str]:
    """Validate an integer monetary value without performing any conversion."""
    if isinstance(value, bool) or not isinstance(value, int):
        return False, "SYJ amount must be an integer number of base units."
    if value < 0 or (value == 0 and not allow_zero):
        return False, "SYJ amount must be positive."
    if value > MAX_SUPPLY_BASE_UNITS:
        return False, "SYJ amount exceeds the maximum SYJ supply."
    return True, ""
