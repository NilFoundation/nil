package relayer

import "embed"

//go:generate bash -c "solc ../../../rollup-bridge-contracts/contracts/bridge/libraries/L1BridgeMessengerEvents.sol --abi --overwrite -o ../rollup-bridge-contracts-compiled-abi/contracts/bridge/l1/L1BridgeMessenger.sol --allow-paths .,../../../rollup-bridge-contracts/contracts/common/libraries --no-cbor-metadata --metadata-hash none --pretty-json"
//go:generate bash -c "solc ../../../rollup-bridge-contracts/contracts/bridge/l2/interfaces/IRelayMessage.sol --abi --overwrite -o ../rollup-bridge-contracts-compiled-abi/contracts/bridge/l2/L2BridgeMessenger.sol --allow-paths .,../../../rollup-bridge-contracts/contracts/common/libraries --no-cbor-metadata --metadata-hash none --pretty-json"

var Fs embed.FS
