import { NODE_MODULES } from "./globals";

//startRetailerCompilation
export const RETAILER_COMPILATION_COMMAND = `solc -o ./tests/Retailer --bin --abi ./tests/Retailer.sol --overwrite ${NODE_MODULES}`;
//endRetailerCompilation

//startManufacturerCompilation
export const MANUFACTURER_COMPILATION_COMMAND = `solc -o ./tests/Manufacturer --bin --abi ./tests/Manufacturer.sol --overwrite ${NODE_MODULES}`;
//endManufacturerCompilation

//startCompilation
export const COUNTER_COMPILATION_COMMAND = `solc -o ./tests/Counter --bin --abi ./tests/Counter.sol --overwrite ${NODE_MODULES}`;
//endCompilation

export const RECEIVER_COMPILATION_COMMAND = `solc -o ./tests/Receiver --bin --abi ./tests/Receiver.sol --overwrite ${NODE_MODULES}`;

//startCallerCompilation
export const CALLER_COMPILATION_COMMAND = `solc -o ./tests/Caller --bin --abi ./tests/Caller.sol --overwrite ${NODE_MODULES}`;
//endCallerCompilation

export const CALLER_ASYNC_COMPILATION_COMMAND = `solc -o ./tests/CallerAsync --bin --abi ./tests/CallerAsync.sol --overwrite ${NODE_MODULES}`;

export const CALLER_ASYNC_BP_COMPILATION_COMMAND = `solc -o ./tests/CallerAsyncBasicPattern --bin --abi ./tests/CallerAsyncBasicPattern.sol  --overwrite ${NODE_MODULES}`;

export const ESCROW_COMPILATION_COMMAND = `solc -o ./tests/Escrow --bin --abi ./tests/Escrow.sol  --overwrite ${NODE_MODULES}`;

export const VALIDATOR_COMPILATION_COMMAND = `solc -o ./tests/Validator --bin --abi ./tests/Validator.sol  --overwrite ${NODE_MODULES}`;
export const AWAITER_COMPILATION_COMMAND = `solc -o ./tests/Awaiter --bin --abi ./tests/Awaiter.sol --overwrite ${NODE_MODULES}`;

export const SWAP_MATCH_COMPILATION_COMMAND = `solc -o ./tests/SwapMatch --abi --bin ./tests/SwapMatch.sol --overwrite ${NODE_MODULES}`;

//startCounterBugCompilationCommand
export const COUNTER_BUG_COMPILATION_COMMAND =
  "solc -o ./tests/CounterBug --bin --abi ./tests/CounterBug.sol --overwrite --no-cbor-metadata --metadata-hash none";
//endCounterBugCompilationCommand

export const MULTISIG_COMPILATION_COMMAND = `solc -o ./tests/MultiSigSmartAccount --abi --bin ./tests/MultiSigSmartAccount.sol --overwrite ${NODE_MODULES}`;

export const NFT_COMPILATION_COMMAND = `solc -o ./tests/NFT --abi --bin ./tests/NFT.sol --overwrite ${NODE_MODULES}`;

export const AUCTION_COMPILATION_COMMAND = `solc -o ./tests/EnglishAuction --abi --bin ./tests/EnglishAuction.sol --overwrite ${NODE_MODULES}`;

export const CLONE_FACTORY_COMPILATION_COMMAND = `solc -o ./tests/CloneFactory --abi --bin ./tests/CloneFactory.sol --overwrite ${NODE_MODULES}`;

export const FT_COMPILATION_COMMAND = `solc -o ./tests/FT --abi --bin ./tests/FT.sol --overwrite ${NODE_MODULES}`;
