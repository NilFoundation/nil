// SPDX-License-Identifier: MIT

pragma solidity ^0.8.21;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

/**
 * @title Counter
 * @author =nil; Foundation
 * @notice A counter contract that increments a value.
 */
contract Counter {
    uint256 private value;

    event ValueChanged(uint256 newValue);

    receive() external payable {}

    function increment() public {
        value += 1;
        emit ValueChanged(value);
    }

    function getValue() public view returns (uint256) {
        return value;
    }

    function verifyExternal(
        uint256 hash,
        bytes memory authData
    ) external pure returns (bool) {
        return true;
    }
}

/**
 * @title Deployer
 * @author =nil; Foundation
 * @notice The contract that is meant to deploy Counter.
 */
contract Deployer is NilBase {
    constructor() public payable {}

    function deposit() public payable {}

    /**
     * The function for deploying the Counter contract.
     * @param data The bytecode of the Counter contract.
     */
    function deploy(bytes memory data, uint salt) public payable {
        Nil.asyncDeploy(2, address(0), 0, data, salt);
    }
}
