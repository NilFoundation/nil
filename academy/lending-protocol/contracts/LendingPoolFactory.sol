// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.19;

import "@openzeppelin/contracts/access/Ownable.sol";

contract GlobalLedger is Ownable {
    mapping(address => bool) public registeredPools;
    address public factory;
    
    modifier onlyFactory() {
        require(msg.sender == factory, "Not authorized");
        _;
    }
    
    modifier onlyRegisteredPool() {
        require(registeredPools[msg.sender], "Unauthorized pool");
        _;
    }
    
    constructor(address _factory, address _owner) Ownable(_owner) {
        factory = _factory;
    }
    
    function registerLendingPool(address pool) external onlyFactory {
        registeredPools[pool] = true;
    }
    
    function recordDeposit(address user, uint256 amount) external onlyRegisteredPool {
        // Logic for recording deposit
    }
    
    function recordLoan(address user, uint256 amount) external onlyRegisteredPool {
        // Logic for recording loan
    }
}

contract LendingPool {
    address public globalLedger;
    address public owner;
    
    modifier onlyOwner() {
        require(msg.sender == owner, "Not authorized");
        _;
    }
    
    constructor(address _globalLedger, address _owner) {
        globalLedger = _globalLedger;
        owner = _owner;
    }
    
    function deposit(uint256 amount) external {
        // Deposit logic
        GlobalLedger(globalLedger).recordDeposit(msg.sender, amount);
    }
    
    function borrow(uint256 amount) external {
        // Borrow logic
        GlobalLedger(globalLedger).recordLoan(msg.sender, amount);
    }
}

contract LendingPoolFactory {
    address public globalLedger;
    event LendingPoolDeployed(address pool, address owner);
    
    constructor(address _globalLedger) {
        globalLedger = _globalLedger;
    }
    
    function deployLendingPool() external returns (address) {
        LendingPool newPool = new LendingPool(globalLedger, msg.sender);
        GlobalLedger(globalLedger).registerLendingPool(address(newPool));
        emit LendingPoolDeployed(address(newPool), msg.sender);
        return address(newPool);
    }
}
