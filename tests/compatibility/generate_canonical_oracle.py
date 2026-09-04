#!/usr/bin/env python3
"""Regenerate the deterministic Python canonical-JSON oracle fixture.

The production Go serializer must not invoke Python. This script exists only
for development/compatibility work and records CPython json.dumps output as the
byte-for-byte oracle.
"""
import json
import math
import random
from pathlib import Path

OUT = Path(__file__).with_name("fixtures") / "canonical_python_oracle.json"


def add(cases, name, value):
    cases.append(
        {
            "name": name,
            "value_json": json.dumps(value, ensure_ascii=False, separators=(",", ":")),
            "expected": json.dumps(value, sort_keys=True, separators=(",", ":"), default=str),
        }
    )


def main():
    cases = []
    explicit = [
        0.0, -0.0, 1.0, -1.0, 0.0001,
        0.00009999999999999999, 0.00010000000000000002,
        1e-4, 1e-5, 1e-7, 1e-15, 1e-16,
        1e15, 1e16, 1e16 - 1, 1e16 + 2,
        1e20, 1e-20, 5e-324, 2.2250738585072014e-308,
        1.7976931348623157e308, 1.2345678901234567,
        -1.2345678901234567, 9007199254740992.0,
        9007199254740991.0,
    ]
    for i, value in enumerate(explicit):
        add(cases, f"float_explicit_{i}", value)

    rng = random.Random(20260905)
    for i in range(200):
        exponent = rng.randint(-320, 300)
        value = (rng.random() * 2.0 - 1.0) * (10.0 ** exponent)
        if math.isfinite(value):
            add(cases, f"float_random_{i}", value)

    strings = [
        "", "ascii", "é", "雪", "😀", "line\nfeed", "quote\"",
        "backslash\\", "\b\f\r\t", "\u0000\u001f", "\x7f", "slash/",
        "café\u2028\u2029",
    ]
    for i, value in enumerate(strings):
        add(cases, f"string_{i}", value)

    add(cases, "nested_protocol", {
        "timestamp": 1735689600.0,
        "sender": "SYJabc",
        "receiver": "SYJdef",
        "amount_base_units": 5000000000,
    })
    add(cases, "nested_mixed", {
        "z": [1, 2.0, True, None, "é"],
        "a": {"n": -0.0, "s": "\\\"\n"},
    })
    add(cases, "nested_deep", {
        "outer": {"b": [{"x": -1.0}, {"x": 1e-7}], "a": [0, False]},
        "empty": {},
    })
    add(cases, "array_numbers", [0.0, -0.0, 1.0, 1e-7, 1e16, 2**63 - 1, 2**64 - 1])
    add(cases, "large_ints", {"u": 2**64 - 1, "i": -(2**63), "safe": 2**53})
    add(cases, "booleans_null", {"t": True, "f": False, "n": None})

    OUT.write_text(json.dumps(cases, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {len(cases)} cases to {OUT}")


if __name__ == "__main__":
    main()
