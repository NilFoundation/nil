// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./nil/NilCurrencyBase.sol";
contract IncrementerPayable is NilCurrencyBase {
    uint256 private value;

    event ValueChanged(uint256 newValue);

    constructor() payable {}
    receive() external payable {}

    function increment() public onlyInternal payable{
        value += 1;
        emit ValueChanged(value);
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}

