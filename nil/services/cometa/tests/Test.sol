// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.8.2;

import "./TestLib.sol";

contract Foo {
    function test(bool success, uint b) public payable returns (uint) {
        require(success, "Test failed");
        return TestLib.add(1, b);
    }
}
