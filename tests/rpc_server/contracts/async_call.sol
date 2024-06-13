// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../../../contracts/nil.sol";

contract Callee {
    int32 value;

    function add(int32 val) public returns(int32) {
        value += val;
        return value;
    }
}

contract Caller {
    using nil for address;

    function call(address dst, int32 val) public payable {
        dst.async_call(gasleft(), msg.value, abi.encodeWithSignature("add(int32)", val));
    }

    function send_msg(bytes calldata message) public payable {
        nil.send_msg(gasleft(), message);
    }

    function verifyExternal(bytes memory unused, uint256 hash, bytes memory signature) public view returns (bool) {
        return true;
    }
}
