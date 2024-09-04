// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Caller {
    using Nil for address;

    function call(address dst) public {
        Nil.asyncCall(
            dst,
            msg.sender,
            msg.sender,
            100000,
            0,
            false,
            0,
            abi.encodeWithSignature("increment()")
        );
    }
}

