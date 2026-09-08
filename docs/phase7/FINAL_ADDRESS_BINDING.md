# Final Production Address Binding Gate

The Phase 7 production genesis remains blocked until five intentional final public SYJ addresses are supplied:

1. Presale
2. Treasury
3. Ecosystem/Grants
4. Liquidity
5. Team/Advisors

Populate only the `recipient` fields in `protocol/tokenomics/genesis_allocation.json`, based on the template in the same directory. Do not change categories or amounts.

Every recipient MUST be validated with the existing repository SYJ address validator. Do not replace it with a new address format or regex.

After binding: validate allocation → generate the separately versioned Phase 7 genesis artifact/vector → verify 28,800,000,000,000,000 genesis base units → verify 43,200,000,000,000,000 remaining mining base units → verify 72,000,000,000,000,000 maximum → run all frozen/regression suites → publish the final Phase 7 genesis SHA-256.

Never generate, derive, randomize, or invent an address to bypass this gate.
