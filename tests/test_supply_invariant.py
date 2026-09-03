from blockchain.block import Block
from blockchain.mining import Miner
from blockchain.native_asset import MAX_SUPPLY_BASE_UNITS
from blockchain.transaction import Transaction
from blockchain.validators import validate_block_against_chain, validate_chain
from blockchain.wallet import Wallet


def test_mining_supply_is_integer_and_accounted(blockchain):
    miner = Wallet.create()
    block = blockchain.mine_pending_transactions(miner.address)
    assert isinstance(block.transactions[0].amount_base_units, int)
    assert blockchain.total_supply_base_units() == blockchain.settings.mining.block_reward
    assert blockchain.get_balance(miner.address) == blockchain.settings.mining.block_reward


def test_coinbase_above_remaining_supply_is_rejected(blockchain):
    miner = Wallet.create()
    reward = blockchain.settings.mining.block_reward
    # Build a synthetic valid-looking block with an impossible issuance.
    coinbase = Transaction.new_coinbase(miner.address, MAX_SUPPLY_BASE_UNITS + 1)
    block = Block(
        index=1,
        previous_hash=blockchain.latest_block.hash,
        transactions=[coinbase],
        timestamp=blockchain.latest_block.timestamp + 1,
        difficulty=blockchain.current_difficulty(),
    )
    ok, reason = validate_block_against_chain(
        block, blockchain.chain, blockchain.consensus, blockchain.settings.consensus,
        reward, MAX_SUPPLY_BASE_UNITS,
    )
    assert not ok
    assert "supply" in reason.lower() or "reward" in reason.lower()


def test_state_replay_rejects_unfunded_signed_transfer(blockchain):
    miner = Wallet.create()
    receiver = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)
    attacker = Wallet.create()
    tx = Transaction(attacker.address, receiver.address, "1")
    tx.sign(attacker)
    block = Block(
        index=1,
        previous_hash=blockchain.genesis_block.hash,
        transactions=[Transaction.new_coinbase_base_units(miner.address, blockchain.settings.mining.block_reward), tx],
        timestamp=blockchain.genesis_block.timestamp + 1,
        difficulty=blockchain.settings.consensus.difficulty,
    )
    # No valid PoW is required to establish the monetary-state rejection;
    # the full chain validator will reject the candidate independently.
    ok, reason = validate_chain(
        [blockchain.genesis_block, block], blockchain.consensus,
        blockchain.settings.consensus, blockchain.settings.mining.block_reward,
        MAX_SUPPLY_BASE_UNITS,
    )
    assert not ok


def test_reward_is_capped_at_remaining_supply(tmp_path, monkeypatch):
    from dataclasses import replace
    from config.settings import get_settings
    from blockchain.blockchain import Blockchain

    monkeypatch.setenv("SYJ_DB_FILE", f"test_supply_tail_{__import__('uuid').uuid4().hex}.db")
    get_settings.cache_clear()
    settings = get_settings()
    settings = replace(
        settings,
        native_asset=replace(settings.native_asset, max_supply_base_units=7_500_000_000),
    )
    chain = Blockchain(settings)
    miner = Wallet.create()
    first = chain.mine_pending_transactions(miner.address)
    second = chain.mine_pending_transactions(miner.address)
    assert first.transactions[0].amount_base_units == 5_000_000_000
    assert second.transactions[0].amount_base_units == 2_500_000_000
    assert chain.total_supply_base_units() == 7_500_000_000
    try:
        chain.mine_pending_transactions(miner.address)
        assert False, "Mining should stop once maximum supply is reached."
    except Exception as exc:
        assert "supply" in str(exc).lower()
    get_settings.cache_clear()


def test_transaction_roundtrip_preserves_base_units():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender.address, receiver.address, "0.00000001")
    tx.sign(sender)
    restored = Transaction.from_dict(tx.to_dict())
    assert restored.amount_base_units == 1
    assert restored.tx_hash == tx.tx_hash
    assert restored.verify()
