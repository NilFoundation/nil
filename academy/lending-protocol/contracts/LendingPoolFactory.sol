// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "./LendingPool.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";

/// @title LendingPoolFactory
/// @dev Deploys LendingPool contracts across shards and registers them in GlobalLedger
contract LendingPoolFactory {
    address public globalLedger;
    uint8 public globalLedgerShardId;
    address public interestManager;
    address public oracle;
    TokenId public usdt;
    TokenId public eth;

    uint8 public shardCounter;
    uint256 public saltCounter; //added for uniqueness in deployments

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
        saltCounter = 0;
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

        // Deploy the LendingPool contract to the next shard using salt for uniqueness
        address poolAddress = Nil.asyncDeploy(
            shardCounter,
            msg.sender,
            address(0),
            0,
            0,
            0,
            bytecode,
            saltCounter //dynamic salt value
        );

        //increment salt counter to avoid reuse
        unchecked {
            saltCounter++;
        }

        Nil.asyncCall(
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

        emit LendingPoolDeployed(poolAddress, shardCounter, msg.sender);

        shardCounter = (shardCounter + 1) % 4; // Cycle through 4 shards
    }
}
