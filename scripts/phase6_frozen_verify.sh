#!/bin/sh
set -eu
[ "$(sha256sum protocol/SYJ_PROTOCOL_SPEC.md | cut -d' ' -f1)" = "7c898c90c1308493fc45e43542f80ac9508dab64d5c70377b084689bec97b3d5" ]
[ "$(sha256sum protocol/test-vectors/address.json | cut -d' ' -f1)" = "934bc7f39cfa1f237b6977176ce7a495f1376a4b74c6e055bfe7c0e05772a06c" ]
[ "$(sha256sum protocol/test-vectors/block.json | cut -d' ' -f1)" = "a6a729f8371046c6673b356c0d599bf8aa0f6b9e9cb019441a058d5b61a1e074" ]
[ "$(sha256sum protocol/test-vectors/chain_work.json | cut -d' ' -f1)" = "31c2b2b8de636036b5346c24c9e6317906419b911413aa70329a53478d4e0471" ]
[ "$(sha256sum protocol/test-vectors/difficulty.json | cut -d' ' -f1)" = "899bb814808a30c70e436c297b77d9f035bfa5db0580b0ccff33c1c133aaf2dc" ]
[ "$(sha256sum protocol/test-vectors/genesis.json | cut -d' ' -f1)" = "bcbc4ad94084b4e4cbf9c6247dfda8dccc60a5a42b9f660955fc1c311dbd9434" ]
[ "$(sha256sum protocol/test-vectors/merkle.json | cut -d' ' -f1)" = "48e484876a36fb9e5945c581b626d193499dc122776ace810e3abfdab482202e" ]
[ "$(sha256sum protocol/test-vectors/monetary.json | cut -d' ' -f1)" = "7f502bb4d569bbd5a2a059088394bdd75a090ec63f551bc80065f2b838bb0618" ]
[ "$(sha256sum protocol/test-vectors/pow.json | cut -d' ' -f1)" = "1471889f5b45b2fbb58c6f07525b856d4c844f2d70735c5ef4e599d079fd3b4d" ]
[ "$(sha256sum protocol/test-vectors/transaction.json | cut -d' ' -f1)" = "8729cff6d3012eff82367fea7ba3a1999030c8d5e97bfc7796565b156283a426" ]
echo 'PHASE 4 FROZEN HASHES PASS'
