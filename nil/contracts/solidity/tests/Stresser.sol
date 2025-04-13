// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import "../lib/NilTokenBase.sol";

contract StresserFactory {

    event deployed(address[] contracts);

    function deployContracts(uint256 n, uint256 balance) public {
        address[] memory res = new address[](n);
        for (uint256 i = 0; i < n; i++) {
            res[i] = address(new Stresser{salt: bytes32(abi.encodePacked(i)), value: balance}());
        }
        emit deployed(res);
    }

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
    }
}

contract Stresser {

    uint256 public value;
    uint256[1024*1024] array;
    mapping(uint256 => uint256) public map;

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

    // Consumes gas by using cold SSTORE opcodes(~20331 gas per iteration)
    function gasConsumerColdSSTORE(uint256 n) public returns(uint256) {
        uint256 start = map[0];
        for (uint256 i = start; i < n + start; i++) {
            map[i] = i;
        }
        map[0] = start + n;
        return start + n;
    }

    function asyncCalls(address[] memory addresses, uint256 n) public {
        for (uint256 i = 0; i < addresses.length; i++) {
            gasConsumer(n/addresses.length);
            Nil.asyncCall(
                addresses[i],
                address(0),
                0,
                abi.encodeWithSignature("gasConsumer(uint256)", n)
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
    ) public {
        gasConsumer(10); // 5000 gas
        require(success, "Request failed");
    }

    function factorialAwait(uint32 n, address peer) public returns (uint256) {
        return factorialRec(uint256(n), peer);
    }

    function factorialRec(uint256 n, address peer) public returns (uint256) {
        if (n == 0) {
            return 1;
        }
        if (gasConsumer(10) == n) { // 5000 gas
            n++;
        }
        bytes memory callData = abi.encodeWithSelector(this.factorialRec.selector, n - 1, address(this));
        (bytes memory returnData, bool success) = Nil.awaitCall(peer, Nil.ASYNC_REQUEST_MIN_GAS, callData);
        require(success, "awaitCall failed");
        uint256 prev = abi.decode(returnData, (uint256));
        if (gasConsumer(10) == n) { // 5000 gas
            n++;
        }
        return n * prev;
    }

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
    }
}
