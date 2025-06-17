// SPDX-License-Identifier: MIT
//startContract
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract CounterBug {
    uint256 private value;

    event ValueChanged(uint256 newValue);

    function increment() public {
        require(Nil.msgSender() == address(0));
        value += 1;
        emit ValueChanged(value);
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}

//endContract
