// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.0;

contract Sender {
    function send(address payable _address, uint256 _value) public {
        bool success = _address.send(_value);
        require(success, "Send value failed");
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }
}
