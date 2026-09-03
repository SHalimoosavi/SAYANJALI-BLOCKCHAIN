"""
CLI for SAYANJALI BLOCKCHAIN, built with Typer + Rich.

Run with:
    python -m cli.main --help

Commands:
    create-wallet        Generate a new wallet and print its keys/address.
    show-chain           Print a summary of every block in the chain.
    mine                 Mine a new block, rewarding a given address.
    status               Print node/chain status.
    create-transaction   Build, sign, and submit a transaction end-to-end.
    validate             Validate the full chain and print the result.
    network-status       Print this node's P2P identity and status.
    peers                List known peers.
    add-peer             Register a peer (bidirectional, best-effort).
    sync                 Synchronize the chain from known peers.
    start-node           Start the FastAPI node (equivalent to uvicorn).
"""

from __future__ import annotations

import typer
from rich.console import Console
from rich.table import Table

from blockchain.blockchain import Blockchain
from blockchain.transaction import Transaction
from blockchain.utils import ValidationError
from blockchain.wallet import Wallet, is_valid_address

app = typer.Typer(
    name="sayanjali",
    help="SAYANJALI BLOCKCHAIN command-line interface (SYJ Token network).",
    add_completion=False,
)
console = Console()


def _get_chain() -> Blockchain:
    """Construct a Blockchain instance bound to the on-disk database."""
    return Blockchain()


def _get_network_node(chain: Blockchain | None = None):
    """
    Construct a NetworkNode, optionally bound to an already-constructed
    Blockchain instance (to avoid opening a second, redundant connection
    to the same database within a single command).

    Unlike the API's module-level singleton (which persists for the
    life of a running server process), each CLI invocation is its own
    short-lived process -- so node identity and peer state are recovered
    from the database (see NetworkNode/PeerRegistry persistence) rather
    than kept in memory across commands.
    """
    from blockchain.network.node import NetworkNode

    return NetworkNode(chain or _get_chain())


@app.command("create-wallet")
def create_wallet() -> None:
    """Generate a new wallet and print its address and key material."""
    wallet = Wallet.create()
    console.print("[bold green]New wallet created[/bold green]")
    console.print(f"Address:     [cyan]{wallet.address}[/cyan]")
    console.print(f"Public Key:  {wallet.public_key_hex}")
    console.print(f"Private Key: [red]{wallet.private_key_hex}[/red]")
    console.print(
        "[yellow]Warning:[/yellow] store the private key securely. "
        "It cannot be recovered if lost."
    )


@app.command("show-chain")
def show_chain() -> None:
    """Print a table summarizing every block currently on the chain."""
    chain = _get_chain()
    table = Table(title=f"SAYANJALI BLOCKCHAIN ({chain.settings.network_name})")
    table.add_column("Index", justify="right")
    table.add_column("Hash")
    table.add_column("Previous Hash")
    table.add_column("Tx Count", justify="right")
    table.add_column("Nonce", justify="right")

    for block in chain.chain:
        table.add_row(
            str(block.index),
            block.hash[:16] + "...",
            block.previous_hash[:16] + "...",
            str(len(block.transactions)),
            str(block.nonce),
        )
    console.print(table)
    console.print(f"Total blocks: [bold]{chain.length}[/bold]")


@app.command("mine")
def mine(
    miner_address: str = typer.Argument(..., help="Address to receive the block reward.")
) -> None:
    """Mine a new block, including all pending mempool transactions."""
    if not is_valid_address(miner_address):
        console.print("[bold red]Error:[/bold red] malformed miner address.")
        raise typer.Exit(code=1)

    chain = _get_chain()
    with console.status("[bold green]Mining block..."):
        try:
            block = chain.mine_pending_transactions(miner_address)
        except ValidationError as exc:
            console.print(f"[bold red]Mining failed:[/bold red] {exc}")
            raise typer.Exit(code=1) from exc

    console.print("[bold green]Block mined successfully[/bold green]")
    console.print(f"Index: {block.index}")
    console.print(f"Hash:  {block.hash}")
    console.print(f"Nonce: {block.nonce}")
    console.print(f"Transactions included: {len(block.transactions)}")

    try:
        from blockchain.network.propagation import broadcast_block

        node = _get_network_node(chain)
        acknowledged = broadcast_block(node, block)
        if acknowledged:
            console.print(f"Broadcast to {len(acknowledged)} peer(s).")
    except Exception as exc:  # noqa: BLE001
        console.print(f"[yellow]Block broadcast skipped:[/yellow] {exc}")


@app.command("status")
def status() -> None:
    """Print current node and chain status."""
    chain = _get_chain()
    info = chain.status()
    for key, value in info.items():
        console.print(f"[bold]{key}[/bold]: {value}")


@app.command("create-transaction")
def create_transaction(
    private_key: str = typer.Option(..., prompt=True, hide_input=True, help="Sender's private key."),
    receiver: str = typer.Option(..., help="Receiver's wallet address."),
    amount: str = typer.Option(..., help="Amount of SYJ to send (up to 8 decimal places)."),
) -> None:
    """Build, sign, and submit a transaction to the local node's mempool."""
    try:
        wallet = Wallet.from_private_key(private_key)
    except Exception as exc:  # noqa: BLE001
        console.print(f"[bold red]Invalid private key:[/bold red] {exc}")
        raise typer.Exit(code=1) from exc

    if not is_valid_address(receiver):
        console.print("[bold red]Error:[/bold red] malformed receiver address.")
        raise typer.Exit(code=1)

    tx = Transaction(sender=wallet.address, receiver=receiver, amount=amount)
    tx.sign(wallet)

    chain = _get_chain()
    accepted, reason = chain.submit_transaction(tx)
    if accepted:
        console.print("[bold green]Transaction accepted into mempool[/bold green]")
        console.print(f"tx_hash: {tx.tx_hash}")

        try:
            from blockchain.network.propagation import broadcast_transaction

            node = _get_network_node(chain)
            acknowledged = broadcast_transaction(node, tx)
            if acknowledged:
                console.print(f"Broadcast to {len(acknowledged)} peer(s).")
        except Exception as exc:  # noqa: BLE001
            console.print(f"[yellow]Transaction broadcast skipped:[/yellow] {exc}")
    else:
        console.print(f"[bold red]Transaction rejected:[/bold red] {reason}")
        raise typer.Exit(code=1)


@app.command("validate")
def validate() -> None:
    """Validate the entire chain and print the result."""
    chain = _get_chain()
    is_valid, reason = chain.is_chain_valid()
    if is_valid:
        console.print("[bold green]Chain is VALID[/bold green]")
    else:
        console.print(f"[bold red]Chain is INVALID:[/bold red] {reason}")
        raise typer.Exit(code=1)


@app.command("network-status")
def network_status() -> None:
    """Print this node's networking identity and summary status."""
    node = _get_network_node()
    for key, value in node.status().items():
        console.print(f"[bold]{key}[/bold]: {value}")


@app.command("peers")
def peers() -> None:
    """List every peer this node currently knows about."""
    node = _get_network_node()
    known = node.peers.list_peers()

    table = Table(title="Known Peers")
    table.add_column("Address")
    table.add_column("Node ID")
    table.add_column("Status")
    table.add_column("Last Seen", justify="right")

    for peer in known:
        table.add_row(
            peer.address,
            (peer.node_id or "-")[:12],
            peer.status,
            f"{peer.last_seen:.0f}" if peer.last_seen else "-",
        )
    console.print(table)
    console.print(f"Total peers: [bold]{len(known)}[/bold]")


@app.command("add-peer")
def add_peer(
    address: str = typer.Argument(..., help="Peer address, e.g. http://127.0.0.1:8001")
) -> None:
    """
    Register a peer for discovery, then cryptographically authenticate
    with it.

    Discovery registration alone never grants trust (per Phase 6.5's
    security model) -- only a successfully completed challenge-response
    handshake does. This command performs both: it registers this node
    with the peer for discovery, then proves this node's identity to the
    peer via the handshake, which makes *the peer* trust *this node*.

    To establish full mutual trust (required before either side will
    accept propagated blocks/transactions from the other, or accept the
    other as a sync target), the operator on the peer's side needs to run
    the equivalent command pointed back at this node:
        python -m cli.main add-peer <this-node's-advertised-address>
    """
    from blockchain.network.handshake import perform_handshake

    node = _get_network_node()
    accepted, reason = node.peers.register(address)
    if not accepted:
        console.print(f"[bold red]Could not add peer:[/bold red] {reason}")
        raise typer.Exit(code=1)

    console.print(f"[bold green]Peer added for discovery:[/bold green] {address}")

    result = node.client.register_with(address, node.node_id, node.self_address)
    if result is not None:
        node.peers.mark_seen(address, "online", result.get("self_node_id"))
        console.print("Peer acknowledged discovery registration.")
    else:
        node.peers.mark_seen(address, "offline")
        console.print(
            "[yellow]Peer did not respond to discovery registration.[/yellow]"
        )

    console.print("Attempting cryptographic authentication...")
    auth_ok, auth_reason = perform_handshake(node, address)
    if auth_ok:
        console.print(
            "[bold green]Authenticated:[/bold green] the peer now trusts this node."
        )
        console.print(
            "[dim]For full mutual trust, run the equivalent add-peer command on "
            f"the peer's side, pointed at {node.self_address}[/dim]"
        )
    else:
        console.print(f"[yellow]Authentication not completed:[/yellow] {auth_reason}")
        console.print(
            "[dim]The peer is registered for discovery but is not yet trusted -- "
            "block/transaction propagation and sync will not work with it until "
            "authentication succeeds.[/dim]"
        )


@app.command("sync")
def sync_command(
    peer: str = typer.Option(
        None, help="Sync from a specific peer only. Defaults to all known peers."
    )
) -> None:
    """Synchronize this node's chain from known peers."""
    from blockchain.network import sync as sync_module

    node = _get_network_node()
    with console.status("[bold green]Synchronizing..."):
        if peer:
            results = [sync_module.sync_with_peer(node, peer)]
        else:
            results = sync_module.sync_with_all_peers(node)

    if not results:
        console.print("[yellow]No known peers to sync with.[/yellow]")
        return

    table = Table(title="Sync Results")
    table.add_column("Peer")
    table.add_column("Accepted")
    table.add_column("Reason")
    table.add_column("Length Before", justify="right")
    table.add_column("Length After", justify="right")

    for result in results:
        table.add_row(
            result.peer_address,
            "[green]yes[/green]" if result.accepted else "[red]no[/red]",
            result.reason or "-",
            str(result.local_length_before),
            str(result.local_length_after),
        )
    console.print(table)


@app.command("start-node")
def start_node(
    host: str = typer.Option(None, help="Override the configured host."),
    port: int = typer.Option(None, help="Override the configured port."),
) -> None:
    """Start the FastAPI node using uvicorn."""
    import uvicorn

    from config.settings import get_settings

    settings = get_settings()
    console.print(
        f"[bold green]Starting SAYANJALI BLOCKCHAIN node[/bold green] "
        f"on {host or settings.host}:{port or settings.port}"
    )
    uvicorn.run(
        "api.main:app",
        host=host or settings.host,
        port=port or settings.port,
        reload=False,
    )


if __name__ == "__main__":
    app()
