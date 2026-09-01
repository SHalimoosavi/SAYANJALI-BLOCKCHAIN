"""
Phase 1 tests: mempool pending-spend accounting / double-spend prevention.

Directly re-verifies the exact scenario reproduced live during the Phase 0
audit (confirmed balance 50, two 40-unit spends both accepted, resulting
in a confirmed balance of -30 that is_chain_valid() still reported as
valid), and covers the broader adversarial matrix the directive requires.
"""

from __future__ import annotations

import threading

from blockchain.blockchain import Blockchain
from blockchain.transaction import Transaction
from blockchain.wallet import Wallet


def _fund(blockchain: Blockchain, wallet: Wallet, blocks: int = 1) -> None:
    for _ in range(blocks):
        blockchain.mine_pending_transactions(wallet.address)


def test_two_conflicting_pending_spends_second_rejected(blockchain: Blockchain):
    """The exact Phase 0 reproduction: confirmed=50, two 40-unit spends -- only the first is accepted."""
    sender = Wallet.create()
    receiver_a = Wallet.create()
    receiver_b = Wallet.create()
    _fund(blockchain, sender)  # 50 confirmed

    tx_a = Transaction(sender=sender.address, receiver=receiver_a.address, amount=40.0)
    tx_a.sign(sender)
    accepted_a, _ = blockchain.submit_transaction(tx_a)
    assert accepted_a

    tx_b = Transaction(sender=sender.address, receiver=receiver_b.address, amount=40.0)
    tx_b.sign(sender)
    accepted_b, reason_b = blockchain.submit_transaction(tx_b)
    assert not accepted_b
    assert "pending" in reason_b.lower() or "insufficient" in reason_b.lower()


def test_confirmed_chain_never_reaches_negative_balance(blockchain: Blockchain):
    """
    Direct regression test for the exact live-reproduced Phase 0 exploit:
    even if both transactions were somehow submitted, mining must never
    produce a negative confirmed balance, and the resulting chain must be
    valid. With the mempool fix, TX-B is rejected at submission, so this
    also verifies the failure mode is closed at the correct layer.
    """
    sender = Wallet.create()
    receiver_a = Wallet.create()
    receiver_b = Wallet.create()
    other_miner = Wallet.create()
    _fund(blockchain, sender)

    tx_a = Transaction(sender=sender.address, receiver=receiver_a.address, amount=40.0)
    tx_a.sign(sender)
    blockchain.submit_transaction(tx_a)

    tx_b = Transaction(sender=sender.address, receiver=receiver_b.address, amount=40.0)
    tx_b.sign(sender)
    accepted_b, _ = blockchain.submit_transaction(tx_b)
    assert not accepted_b  # closed at submission, never reaches mining

    blockchain.mine_pending_transactions(other_miner.address)

    final_balance = blockchain.get_balance(sender.address)
    assert final_balance >= 0
    is_valid, reason = blockchain.is_chain_valid()
    assert is_valid, reason


def test_three_way_overspend_only_first_two_fit(blockchain: Blockchain):
    sender = Wallet.create()
    receivers = [Wallet.create() for _ in range(3)]
    _fund(blockchain, sender)  # 50 confirmed

    results = []
    for receiver in receivers:
        tx = Transaction(sender=sender.address, receiver=receiver.address, amount=20.0)
        tx.sign(sender)
        accepted, _ = blockchain.submit_transaction(tx)
        results.append(accepted)

    # 20 + 20 = 40 <= 50 fits; +20 more = 60 > 50 does not.
    assert results == [True, True, False]


def test_exact_balance_spend_accepted(blockchain: Blockchain):
    """Spending exactly the confirmed balance (boundary, not overspend) must be accepted."""
    sender = Wallet.create()
    receiver = Wallet.create()
    _fund(blockchain, sender)
    reward = blockchain.settings.mining.block_reward

    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=reward)
    tx.sign(sender)
    accepted, reason = blockchain.submit_transaction(tx)
    assert accepted, reason


def test_exact_balance_spend_then_any_more_rejected(blockchain: Blockchain):
    sender = Wallet.create()
    receiver_a = Wallet.create()
    receiver_b = Wallet.create()
    _fund(blockchain, sender)
    reward = blockchain.settings.mining.block_reward

    tx_a = Transaction(sender=sender.address, receiver=receiver_a.address, amount=reward)
    tx_a.sign(sender)
    assert blockchain.submit_transaction(tx_a)[0]

    tx_b = Transaction(sender=sender.address, receiver=receiver_b.address, amount=0.01)
    tx_b.sign(sender)
    accepted_b, _ = blockchain.submit_transaction(tx_b)
    assert not accepted_b


def test_pending_spend_followed_by_valid_remaining_spend_accepted(blockchain: Blockchain):
    """After a pending spend, a smaller transaction that fits within the remaining balance is still accepted."""
    sender = Wallet.create()
    receiver_a = Wallet.create()
    receiver_b = Wallet.create()
    _fund(blockchain, sender)  # 50 confirmed

    tx_a = Transaction(sender=sender.address, receiver=receiver_a.address, amount=40.0)
    tx_a.sign(sender)
    assert blockchain.submit_transaction(tx_a)[0]

    tx_b = Transaction(sender=sender.address, receiver=receiver_b.address, amount=10.0)
    tx_b.sign(sender)
    accepted_b, reason_b = blockchain.submit_transaction(tx_b)
    assert accepted_b, reason_b  # 40 + 10 = 50, exactly fits


def test_duplicate_transaction_rejected(blockchain: Blockchain):
    """Regression: identical-hash duplicate submission remains rejected (unrelated to the new pending-spend check)."""
    sender = Wallet.create()
    receiver = Wallet.create()
    _fund(blockchain, sender)

    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=5.0)
    tx.sign(sender)
    assert blockchain.submit_transaction(tx)[0]

    accepted_again, reason = blockchain.submit_transaction(tx)
    assert not accepted_again


def test_conflicting_transaction_from_different_sender_unaffected(blockchain: Blockchain):
    """Pending-spend accounting is per-sender; another address's spending is never blocked by someone else's pending transactions."""
    sender_a = Wallet.create()
    sender_b = Wallet.create()
    receiver = Wallet.create()
    _fund(blockchain, sender_a)
    _fund(blockchain, sender_b)

    tx_a = Transaction(sender=sender_a.address, receiver=receiver.address, amount=50.0)
    tx_a.sign(sender_a)
    assert blockchain.submit_transaction(tx_a)[0]

    tx_b = Transaction(sender=sender_b.address, receiver=receiver.address, amount=50.0)
    tx_b.sign(sender_b)
    accepted_b, reason_b = blockchain.submit_transaction(tx_b)
    assert accepted_b, reason_b


def test_confirmation_removes_pending_reservation(blockchain: Blockchain):
    """Once a transaction is mined, its amount no longer counts as pending, freeing the sender to spend again."""
    sender = Wallet.create()
    receiver_a = Wallet.create()
    receiver_b = Wallet.create()
    other_miner = Wallet.create()
    _fund(blockchain, sender, blocks=2)  # 100 confirmed

    tx_a = Transaction(sender=sender.address, receiver=receiver_a.address, amount=80.0)
    tx_a.sign(sender)
    assert blockchain.submit_transaction(tx_a)[0]
    assert blockchain.mempool.pending_spend_for(sender.address) == 80.0

    blockchain.mine_pending_transactions(other_miner.address)  # confirms tx_a
    assert blockchain.mempool.pending_spend_for(sender.address) == 0.0

    remaining_balance = blockchain.get_balance(sender.address)
    tx_b = Transaction(sender=sender.address, receiver=receiver_b.address, amount=remaining_balance)
    tx_b.sign(sender)
    accepted_b, reason_b = blockchain.submit_transaction(tx_b)
    assert accepted_b, reason_b


def test_rejected_transaction_reserves_nothing(blockchain: Blockchain):
    sender = Wallet.create()
    receiver = Wallet.create()
    _fund(blockchain, sender)

    tx_too_big = Transaction(sender=sender.address, receiver=receiver.address, amount=999.0)
    tx_too_big.sign(sender)
    accepted, _ = blockchain.submit_transaction(tx_too_big)
    assert not accepted
    assert blockchain.mempool.pending_spend_for(sender.address) == 0.0


def test_restart_pending_state_is_cleared_not_persisted(blockchain: Blockchain):
    """
    Mempool state (including pending-spend accounting) is in-memory only,
    matching pre-existing mempool semantics -- a restart naturally clears
    pending transactions rather than requiring new migration/persistence
    work for this fix specifically.
    """
    sender = Wallet.create()
    receiver = Wallet.create()
    _fund(blockchain, sender)

    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=10.0)
    tx.sign(sender)
    blockchain.submit_transaction(tx)
    assert blockchain.mempool.pending_spend_for(sender.address) == 10.0

    reloaded = Blockchain(settings=blockchain.settings)
    assert reloaded.mempool.pending_spend_for(sender.address) == 0.0
    # Confirmed balance, unlike mempool state, is fully preserved.
    assert reloaded.get_balance(sender.address) == blockchain.get_balance(sender.address)


def test_concurrent_submission_race_is_closed(blockchain: Blockchain):
    """
    Two threads submitting conflicting overspend transactions for the
    same sender concurrently must not both be accepted -- the
    check-then-add sequence in submit_transaction is now atomic under
    Blockchain.mutation_lock (an RLock, closing the same race class the
    Phase 6.5 concurrency fix addressed for propagation).
    """
    sender = Wallet.create()
    receiver_a = Wallet.create()
    receiver_b = Wallet.create()
    _fund(blockchain, sender)  # 50 confirmed

    tx_a = Transaction(sender=sender.address, receiver=receiver_a.address, amount=40.0)
    tx_a.sign(sender)
    tx_b = Transaction(sender=sender.address, receiver=receiver_b.address, amount=40.0)
    tx_b.sign(sender)

    results: list[bool] = []
    results_lock = threading.Lock()

    def submit(tx):
        accepted, _ = blockchain.submit_transaction(tx)
        with results_lock:
            results.append(accepted)

    threads = [threading.Thread(target=submit, args=(tx,)) for tx in (tx_a, tx_b)]
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=10)

    assert len(results) == 2
    assert sum(results) == 1  # exactly one of the two conflicting spends was accepted
    assert blockchain.get_balance(sender.address) - blockchain.mempool.pending_spend_for(sender.address) >= 0


def test_reorg_preserves_confirmed_balance_authority(blockchain: Blockchain):
    """
    Reorg behavior for orphaned mempool transactions is explicitly out of
    scope for this minimal fix (existing behavior preserved) -- this test
    documents and verifies that a reorg still leaves confirmed balances
    as the sole authority and the resulting chain valid, which is what
    Phase 1 actually requires here.
    """
    from blockchain.mining import Miner
    from blockchain.validators import expected_difficulty

    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)

    other_miner = Wallet.create()
    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    candidate = [blockchain.chain[0]]
    for i in range(1, 4):
        required = expected_difficulty(candidate, blockchain.consensus, blockchain.settings.consensus)
        new_block, _ = engine.mine_block(
            index=i, previous_hash=candidate[-1].hash, mempool=blockchain.mempool,
            miner_address=other_miner.address, difficulty=required,
        )
        candidate.append(new_block)

    accepted, reason = blockchain.replace_chain(candidate)
    assert accepted, reason
    assert blockchain.get_balance(miner.address) == 0.0  # original miner's reward was on the discarded fork

    is_valid, reason = blockchain.is_chain_valid()
    assert is_valid, reason
