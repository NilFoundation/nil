// SPDX-License-Identifier: MIT

//startEscrow
pragma solidity ^0.8.9;

import "./Nil.sol";

contract Escrow {
    using Nil for address;
    mapping(address => uint256) private deposits;

    function deposit() public payable {
        deposits[msg.sender] += msg.value;
    }

    function submitForVerification(
        address dst,
        address participantOne,
        address participantTwo
    ) public payable {
        dst.asyncCall(
            address(0),
            address(0),
            100000,
            Nil.FORWARD_NONE,
            false,
            1000000,
            abi.encodeWithSignature(
                "verify(address, address)",
                participantOne,
                participantTwo
            )
        );
    }

    function resolve(
        address participantOne,
        address participantTwo
    ) public payable {
        deposits[participantOne] -= msg.value;
        deposits[participantTwo] += msg.value;
    }

    function verifyExternal(
        uint256 messageHash,
        bytes calldata authData
    ) external view returns (bool) {
        return true;
    }
}
//endEscrow
