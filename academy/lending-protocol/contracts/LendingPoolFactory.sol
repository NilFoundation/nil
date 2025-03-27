// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "./LendingPool.sol";

/// @title LendingPoolFactory - Deploys new LendingPool contracts and registers them with the GlobalLedger
contract LendingPoolFactory {
    using Nil for address;

    address public globalLedger;
    address public interestManager;
    address public oracle;
    TokenId public usdt;
    TokenId public eth;

    event LendingPoolDeployed(address pool, address owner);

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
    }

    function deployLendingPool() external returns (address) {
        LendingPool newPool = new LendingPool(
            globalLedger,
            interestManager,
            oracle,
            usdt,
            eth
        );
        address poolAddress = address(newPool);

        // Asynchronously register the new lending pool with the GlobalLedger
        globalLedger.asyncCall(
            address(0),
            0,
            abi.encodeWithSignature("registerLendingPool(address)", poolAddress)
        );

        emit LendingPoolDeployed(poolAddress, msg.sender);
        return poolAddress;
    }
}