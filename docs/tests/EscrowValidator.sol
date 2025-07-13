// SPDX-License-Identifier: MIT

pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilAwaitable.sol";

contract Escrow is NilBase, NilAwaitable {
    using Nil for address;
    mapping(address => uint256) private deposits;

    function deposit() public payable {
        deposits[Nil.msgSender()] += msg.value;
    }

    function submitForVerification(
        address validator,
        address participantOne,
        address participantTwo
    ) public payable async(2_000_000) {
        bytes memory context = abi.encode(
            participantOne,
            participantTwo,
            msg.value
        );
        bytes memory callData = abi.encodeWithSignature(
            "validate(address, address)",
            participantOne,
            participantTwo
        );
        sendRequest(
            validator,
            0,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            callData,
            resolve
        );
    }

    function resolve(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable onlyResponse {
        require(success, "Request failed!");
        (address participantOne, address participantTwo, uint256 value) = abi
            .decode(context, (address, address, uint256));
        bool isValidated = abi.decode(returnData, (bool));
        if (isValidated) {
            deposits[participantOne] -= value;
            deposits[participantTwo] += value;
        }
    }

    function verifyExternal(
        uint256 transactionHash,
        bytes calldata authData
    ) external view returns (bool) {
        return true;
    }
}

contract Validator {
    using Nil for address;

    function validate(
        address participantOne,
        address participantTwo
    ) public returns (bool) {
        bool isValidated = (participantOne != participantTwo);
        return isValidated;
    }
}
