// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilAwaitable.sol";

contract SendRequest is NilAwaitable {
    using Nil for address;

    mapping(string => uint256) public values;

    function call(address to, string memory key) public payable {
        bytes memory context = abi.encode(key);
        bytes memory callData = abi.encodeWithSignature("getValue()");
        sendRequest(to, 0, Nil.ASYNC_REQUEST_MIN_GAS, context, callData, resolve);
    }

    function resolve(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        require(success, "SendRequest: call failed");
        (string memory from) = abi.decode(context, (string));
        uint256 value = abi.decode(returnData, (uint256));
        values[from] = value;
    }
}