from pathlib import Path
import json
from reference_codec import hello, get_peers, peers, get_headers, headers, get_blocks, blocks, new_block, new_tx, reject

def test_reference_vectors_are_self_consistent():
    expected = {
        "hello": hello(), "hello_ack": __import__('reference_codec').hello(True),
        "get_peers": get_peers(), "peers": peers(), "get_headers": get_headers(),
        "headers": headers(), "get_blocks": get_blocks(), "blocks": blocks(),
        "new_block": new_block(), "new_transaction": new_tx(), "reject": reject(),
    }
    data = json.loads((Path(__file__).parent / "fixtures" / "valid.json").read_text())
    assert {k: v["frame_hex"] for k,v in data.items()} == {k: b.hex() for k,b in expected.items()}
