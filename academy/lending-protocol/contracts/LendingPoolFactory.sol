// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "./LendingPool.sol";
import "./INilAsync.sol";

/// @title LendingPoolFactory
/// @dev Handles deployment of LendingPool contracts across different shards and registers them with the GlobalLedger
contract LendingPoolFactory {
    address public globalLedger;
    uint8 public globalLedgerShardId;
    address public interestManager;
    address public oracle;
    TokenId public usdt;
    TokenId public eth;
    uint8 public shardCounter;

    /// @dev Address of the Nil precompile that handles async calls (replace if required)
    address constant NIL_ASYNC_PRECOMPILE =
        0x0000000000000000000000000000000000008003;

    event LendingPoolDeployed(address pool, uint8 shardId, address owner);

    constructor(
        address _globalLedger,
        uint8 _globalLedgerShardId,
        address _interestManager,
        address _oracle,
        TokenId _usdt,
        TokenId _eth
    ) {
        globalLedger = _globalLedger;
        globalLedgerShardId = _globalLedgerShardId;
        interestManager = _interestManager;
        oracle = _oracle;
        usdt = _usdt;
        eth = _eth;
        shardCounter = 0;
    }

    function deployLendingPool() external {
        bytes memory constructorArgs = abi.encode(
            globalLedger,
            interestManager,
            oracle,
            usdt,
            eth,
            msg.sender
        );

        bytes memory bytecode = bytes.concat(
            type(LendingPool).creationCode,
            constructorArgs
        );

        address poolAddress = INilAsync(NIL_ASYNC_PRECOMPILE).asyncDeploy(
            shardCounter,
            msg.sender,
            address(0),
            0,
            0,
            0,
            bytecode,
            0
        );

        require(poolAddress != address(0), "Deployment failed");

        bool success = INilAsync(NIL_ASYNC_PRECOMPILE).asyncCall(
            globalLedgerShardId,
            address(0),
            globalLedger,
            0,
            0,
            0,
            abi.encodeWithSignature(
                "registerLendingPool(address)",
                poolAddress
            ),
            0
        );

        require(success, "asyncCall to GlobalLedger failed");

        emit LendingPoolDeployed(poolAddress, shardCounter, msg.sender);

        shardCounter = (shardCounter + 1) % 4;
    }
}
