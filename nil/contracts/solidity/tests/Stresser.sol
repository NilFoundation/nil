// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import "../lib/NilTokenBase.sol";

contract Stresser {

    uint256 public value;

    constructor() payable {}

    function add(uint256 v) public returns(uint256) {
        value += v;
        return value;
    }

    // Consumes gas by using hot SSTORE(~529 gas per iteration)
    function gasConsumer(uint256 v) public returns(uint256) {
        for (uint256 i = 1; i < v; i++) {
            value *= 2;
        }
        return value;
    }

    function sendTransactions(address[] memory addresses, uint256 v) public {
        for (uint256 i = 0; i < addresses.length; i++) {
            Nil.asyncCall(
                addresses[i],
                address(0),
                0,
                abi.encodeWithSignature("geometricSeq(uint256)", v)
            );
        }
    }

    function sendRequests(address[] memory addresses, uint256 v) public {
        bytes memory context = abi.encodeWithSelector(this.sendRequestsResponse.selector);
        bytes memory callData = abi.encodeWithSignature("gasConsumer(uint256)", v);
        for (uint256 i = 0; i < addresses.length; i++) {
            Nil.sendRequest(
                addresses[i],
                0,
                Nil.ASYNC_REQUEST_MIN_GAS,
                context,
                callData
            );
        }
    }

    function sendRequestsResponse(
        bool success,
        bytes memory,
        bytes memory
    ) public pure {
        require(success, "Request failed");
    }

    function factorialAwait(uint32 n, address peer) public returns (uint256) {
        return factorialRec(uint256(n), peer);
    }

    function factorialRec(uint256 n, address peer) public returns (uint256) {
        if (n == 0) {
            return 1;
        }
        bytes memory callData = abi.encodeWithSelector(this.factorialRec.selector, n - 1, address(this));
        (bytes memory returnData, bool success) = Nil.awaitCall(peer, Nil.ASYNC_REQUEST_MIN_GAS, callData);
        require(success, "awaitCall failed");
        uint256 prev = abi.decode(returnData, (uint256));
        return n * prev;
    }

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
    }
}
