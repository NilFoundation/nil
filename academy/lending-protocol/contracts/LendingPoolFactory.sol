// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "./LendingPool.sol"; // Import LendingPool contract

/// @title LendingPoolFactory
/// @dev Handles deployment of LendingPool contracts across different shards
contract LendingPoolFactory {
    address public globalLedger;
    address public interestManager;
    address public oracle;
    TokenId public usdt;
    TokenId public eth;
    uint8 public shardCounter; // Tracks which shard to deploy to (0-3)

    event LendingPoolDeployed(address pool, uint8 shardId, address owner);

    constructor(
        address _globalLedger,
        address _interestManager,
        address _oracle,
        TokenId _usdt,
        TokenId _eth
    ) {
        globalLedger = _globalLedger;
        interestManager = _interestManager;
        oracle = _oracle;
        usdt = _usdt;
        eth = _eth;
        shardCounter = 0;
    }

    /// @notice Deploys a new LendingPool contract to a shard
    function deployLendingPool() external {
        bytes memory constructorArgs = abi.encode(
            globalLedger,
            interestManager,
            oracle,
            usdt,
            eth,
            msg.sender
        );

        // Compute full contract creation bytecode
        bytes memory bytecode = bytes.concat(
            type(LendingPool).creationCode,
            constructorArgs
        );

        // Deploy LendingPool asynchronously to selected shard
        address poolAddress = Nil.asyncDeploy(
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

        // Cross-shard async call to GlobalLedger to register the new LendingPool
        Nil.asyncCall(
            0, // Shard ID for GlobalLedger
            address(0), // Refund address (not needed here)
            globalLedger, // Target: GlobalLedger contract
            0, // Fee credit
            0, // Forward kind
            0, // ETH value to send
            abi.encodeWithSignature(
                "registerLendingPool(address)",
                poolAddress
            ),
            0 // Salt
        );

        emit LendingPoolDeployed(poolAddress, shardCounter, msg.sender);

        // Cycle through shards
        shardCounter = (shardCounter + 1) % 4;
    }
}
