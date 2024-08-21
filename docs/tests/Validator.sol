// SPDX-License-Identifier: MIT

//startValidator
pragma solidity ^0.8.9;

import "./Nil.sol";
contract Validator {
    using Nil for address;

    function verify(
        address participantOne,
        address participantTwo
    ) public payable {
        if (participantOne != participantTwo) {
            Nil.asyncCall(
                msg.sender,
                address(0),
                address(0),
                100000,
                1,
                false,
                msg.value,
                abi.encodeWithSignature(
                    "resolve(address, address)",
                    participantOne,
                    participantTwo
                )
            );
        }
    }
}
//endValidator
