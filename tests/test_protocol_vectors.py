"""Regression tests for the frozen SYJ protocol compatibility vectors."""

from scripts.protocol_vectors.verify import (
    verify_address,
    verify_block_pow,
    verify_difficulty,
    verify_genesis,
    verify_merkle,
    verify_monetary,
    verify_tx,
    verify_work,
)


def test_genesis_vector():
    verify_genesis()


def test_address_vector():
    verify_address()


def test_transaction_vector():
    verify_tx()


def test_merkle_vectors():
    verify_merkle()


def test_block_and_pow_vectors():
    verify_block_pow()


def test_difficulty_vectors():
    verify_difficulty()


def test_chain_work_vectors():
    verify_work()


def test_monetary_vectors():
    verify_monetary()
