// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract SimpleContract {
    uint256 public value = 42;

    constructor() payable {}

    receive() external payable {}

    function setValue(uint256 _value) public {
        value = _value;
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}

contract Caller {
    constructor() payable {}

    function callSet(address payable addr, uint256 value) public {
        SimpleContract(addr).setValue(value);
    }
    function callSetAndRevert(address payable addr, uint256 value) public {
        SimpleContract(addr).setValue(value);
        revert();
    }
}
