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
    amount: float = typer.Option(..., help="Amount of SYJ to send."),
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
