# Phase 6 Recovery Procedure

1. Stop the node cleanly if possible.
2. Preserve the entire data directory before troubleshooting.
3. Restart normally.
4. Startup must reload the same identity and replay the journal.
5. A journal checksum/format error causes startup failure; do not delete or rewrite the journal to bypass the error.
6. If a valid journal is available, the active tip record determines the canonical chain and all stored branches remain available to the chain manager.

A corrupted identity is also a hard startup failure. The node never silently generates a replacement identity over an existing identity file.
