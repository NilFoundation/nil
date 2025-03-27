// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/// @title Lending System - Unified contract for lending pools and global ledger management
contract LendingSystem is Ownable {
    using Nil for address;

    mapping(address => bool) public registeredPools;
    address public factory;
    mapping(address => uint256) public userDeposits;
    mapping(address => uint256) public userLoans;

    event PoolRegistered(address indexed pool);
    event DepositRecorded(address indexed user, uint256 amount);
    event LoanRecorded(address indexed user, uint256 amount);
    event LendingPoolDeployed(address pool, address owner);

    modifier onlyFactory() {
        require(msg.sender == factory, "Not authorized");
        _;
    }

    constructor(address _owner) Ownable(_owner) {}

    function setFactory(address _factory) external onlyOwner {
        require(factory == address(0), "Factory already set");
        factory = _factory;
    }

    function registerLendingPool(address pool) external onlyFactory {
        registeredPools[pool] = true;
        emit PoolRegistered(pool);
    }

    function recordDeposit(address user, uint256 amount) external {
        require(registeredPools[msg.sender], "Unauthorized pool");
        userDeposits[user] += amount;
        emit DepositRecorded(user, amount);
    }

    function recordLoan(address user, uint256 amount) external {
        require(registeredPools[msg.sender], "Unauthorized pool");
        userLoans[user] += amount;
        emit LoanRecorded(user, amount);
    }

    function getDeposit(address user) external view returns (uint256) {
        return userDeposits[user];
    }

    function deployLendingPool() external {
        // Deploy the LendingPool contract
        LendingPool newPool = new LendingPool(address(this), msg.sender);
        address poolAddress = address(newPool);

        // Async call to register the pool
        bytes memory registerCallData = abi.encodeWithSignature(
            "registerLendingPool(address)",
            poolAddress
        );
        Nil.asyncCall(address(this), address(this), 0, registerCallData);

        emit LendingPoolDeployed(poolAddress, msg.sender);
    }
}

/// @title LendingPool - Individual lending pools for decentralized lending
contract LendingPool {
    using Nil for address;

    address public globalLedger;
    address public owner;

    constructor(address _globalLedger, address _owner) {
        globalLedger = _globalLedger;
        owner = _owner;
    }

    function deposit() public payable {
        Nil.Token[] memory tokens = Nil.txnTokens();
        require(tokens.length > 0 && tokens[0].amount > 0, "Invalid deposit");

        bytes memory callData = abi.encodeWithSignature(
            "recordDeposit(address,uint256)",
            msg.sender,
            tokens[0].amount
        );
        Nil.asyncCall(globalLedger, address(this), 0, callData);
    }

    function borrow(uint256 amount) public payable {
        bytes memory callData = abi.encodeWithSignature("getDeposit(address)", msg.sender);
        bytes memory context = abi.encodeWithSelector(
            this.processLoan.selector,
            msg.sender,
            amount
        );
        Nil.sendRequest(globalLedger, 0, 6_000_000, context, callData);
    }

    function processLoan(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        require(success, "Failed to fetch deposit balance");
        (address borrower, uint256 amount) = abi.decode(context, (address, uint256));
        uint256 userDeposit = abi.decode(returnData, (uint256));
        require(userDeposit >= amount, "Insufficient collateral");

        bytes memory recordLoanCallData = abi.encodeWithSignature(
            "recordLoan(address,uint256)",
            borrower,
            amount
        );
        Nil.asyncCall(globalLedger, address(this), 0, recordLoanCallData);
    }
}