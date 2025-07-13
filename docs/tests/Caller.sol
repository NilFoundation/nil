// SPDX-License-Identifier: MIT

//startContract
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Caller is NilBase {
    using Nil for address;

    receive() external payable {}

    function call(address dst) public async(2_000_000) {
        Nil.asyncCall(
            dst,
            Nil.msgSender(),
            0,
            abi.encodeWithSignature("increment()")
        );
    }

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
    }
}

//endContract
