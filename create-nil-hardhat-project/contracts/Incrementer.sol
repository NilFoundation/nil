// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Incrementer is NilBase {
    uint256 private value;

    event ValueChanged(uint256 newValue);

    function increment() public onlyInternal {
        value += 1;
        emit ValueChanged(value);
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}

