// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.8.2;

import "./TestLib.sol";

contract Foo {
    function test(bool success, uint b, uint unused) public payable returns (uint) {
        require(success, "Test failed");
        return TestLib.add(1, b);
    }

    function makeFail() public pure returns (int32) {
        return abi.decode(bytes(""), (int32));
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }
}
