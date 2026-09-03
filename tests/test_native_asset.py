from decimal import Decimal

import pytest

from blockchain.block import Block
from blockchain.native_asset import (
    BASE_UNITS_PER_SYJ,
    MAX_SUPPLY_BASE_UNITS,
    format_amount,
    from_base_units,
    to_base_units,
)
from blockchain.transaction import Transaction
from blockchain.utils import ValidationError
from blockchain.wallet import Wallet


def test_native_asset_precision_and_roundtrip():
    assert BASE_UNITS_PER_SYJ == 100_000_000
    assert to_base_units("1.23456789") == 123_456_789
    assert from_base_units(123_456_789) == Decimal("1.23456789")
    assert format_amount(123_456_789) == "1.23456789"


def test_native_asset_rejects_excess_precision():
    with pytest.raises(ValueError):
        to_base_units("1.000000001")


def test_transaction_uses_integer_base_units():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender.address, receiver.address, "2.5")
    assert isinstance(tx.amount_base_units, int)
    assert tx.amount_base_units == 250_000_000
    assert tx.amount_syj == Decimal("2.5")


def test_malformed_monetary_value_rejected():
    with pytest.raises(ValidationError):
        Transaction("a", "b", "not-a-number")


def test_max_supply_constant_is_exact():
    assert MAX_SUPPLY_BASE_UNITS == 720_000_000 * BASE_UNITS_PER_SYJ


def test_zero_and_negative_amounts_fail_validation():
    sender = Wallet.create()
    receiver = Wallet.create()
    assert not Transaction(sender.address, receiver.address, "0").verify()
    assert not Transaction(sender.address, receiver.address, "-1").verify()


def test_coinbase_cannot_claim_more_than_supply():
    receiver = Wallet.create()
    tx = Transaction.new_coinbase(receiver.address, MAX_SUPPLY_BASE_UNITS + 1)
    assert not tx.verify()
