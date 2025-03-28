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

    constructor(address _globalLedger, address _interestManager, address _oracle, TokenId _usdt, TokenId _eth) {
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
            msg.sender // Pool owner
        );

        // Compute full contract creation bytecode
        bytes memory bytecode = bytes.concat(type(LendingPool).creationCode, constructorArgs);

        // Call asyncDeploy with correct parameters
        address poolAddress = Nil.asyncDeploy(
            shardCounter,   // Shard ID
            msg.sender,     // Refund to the sender
            address(0),     // Bounce to (set to 0 for now)
            0,              // Fee credit (set to 0 unless required)
            0,              // Forward kind (set to 0 unless forwarding behavior is needed)
            0,              // Value (set to 0 unless ETH needs to be sent)
            bytecode,       // Contract creation code + constructor args
            0               // Salt (set to 0; can be changed for deterministic addresses)
        );

        require(poolAddress != address(0), "Deployment failed");

        emit LendingPoolDeployed(poolAddress, shardCounter, msg.sender);
        shardCounter = (shardCounter + 1) % 4; // Cycle through shards
    }
}