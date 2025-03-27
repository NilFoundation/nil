// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

// Axelar Gateway Interface for cross-chain communication
interface IAxelarGateway {
    function callContract(
        string calldata destinationChain,
        string calldata destinationAddress,
        bytes calldata payload
    ) external;
}

/// @title GlobalLedger - Cross-chain ledger for lending pools
contract GlobalLedger is Ownable {
    mapping(address => bool) public registeredPools;
    address public factory;
    mapping(address => uint256) public userDeposits;
    mapping(address => uint256) public userLoans;

    event PoolRegistered(address indexed pool);
    event DepositRecorded(address indexed user, uint256 amount);
    event LoanRecorded(address indexed user, uint256 amount);

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

    function _execute(bytes calldata payload) internal {
        (address user, uint256 amount, bool isDeposit) = abi.decode(payload, (address, uint256, bool));

        require(registeredPools[msg.sender], "Unauthorized pool");

        if (isDeposit) {
            userDeposits[user] += amount;
            emit DepositRecorded(user, amount);
        } else {
            userLoans[user] += amount;
            emit LoanRecorded(user, amount);
        }
    }
}

contract LendingPool {
    address public globalLedger;
    address public owner;
    string public globalLedgerChain;
    IAxelarGateway public axelarGateway;
    address public token; // Supported token for this lending pool

    event DepositSent(address indexed user, uint256 amount);
    event LoanSent(address indexed user, uint256 amount);

    modifier onlyOwner() {
        require(msg.sender == owner, "Not authorized");
        _;
    }

    constructor(
        address _globalLedger,
        string memory _globalLedgerChain,
        address _gateway,
        address _owner,
        address _token
    ) {
        globalLedger = _globalLedger;
        globalLedgerChain = _globalLedgerChain;
        axelarGateway = IAxelarGateway(_gateway);
        owner = _owner;
        token = _token;
    }

    function deposit(uint256 amount) external {
        bytes memory payload = abi.encode(msg.sender, amount, true);
        string memory globalLedgerAddress = toAsciiString(globalLedger);

        axelarGateway.callContract(globalLedgerChain, globalLedgerAddress, payload);

        emit DepositSent(msg.sender, amount);
    }

    function borrow(uint256 amount) external {
        bytes memory payload = abi.encode(msg.sender, amount, false);
        string memory globalLedgerAddress = toAsciiString(globalLedger);

        axelarGateway.callContract(globalLedgerChain, globalLedgerAddress, payload);

        emit LoanSent(msg.sender, amount);
    }

    function toAsciiString(address _addr) internal pure returns (string memory) {
        bytes memory s = new bytes(42);
        bytes memory hexChars = "0123456789abcdef";

        s[0] = "0";
        s[1] = "x";

        for (uint i = 0; i < 20; i++) {
            s[2 + i * 2] = hexChars[uint8(uint160(_addr) >> (8 * (19 - i)) + 4) & 0xF];
            s[3 + i * 2] = hexChars[uint8(uint160(_addr) >> (8 * (19 - i))) & 0xF];
        }
        return string(s);
    }
}

contract LendingPoolFactory {
    address public globalLedger;
    event LendingPoolDeployed(address pool, address owner, address token);

    constructor(address _globalLedger) {
        globalLedger = _globalLedger;
    }

    function deployLendingPool(
        string memory lendingPoolChain,
        address gateway,
        address token
    ) external returns (address) {
        LendingPool newPool = new LendingPool(globalLedger, lendingPoolChain, gateway, msg.sender, token);
        GlobalLedger(globalLedger).registerLendingPool(address(newPool));
        emit LendingPoolDeployed(address(newPool), msg.sender, token);
        return address(newPool);
    }
}
