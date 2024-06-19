// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../../../contracts/solidity/Nil.sol";

contract Callee {
    int32 value;

    function add(int32 val) public returns (int32) {
        value += val;
        return value;
    }
}

contract Caller {
    using Nil for address;

    function call(address dst, int32 val) public payable {
        dst.async_call(
            address(0),
            gasleft(),
            false,
            msg.value,
            abi.encodeWithSignature("add(int32)", val)
        );
    }

    function send_msg(bytes calldata message) public payable {
        Nil.send_msg(gasleft(), message);
    }

    function verifyExternal(
        uint256,
        bytes memory
    ) external view returns (bool) {
        return true;
    }
}
