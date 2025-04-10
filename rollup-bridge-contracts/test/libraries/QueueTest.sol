// SPDX-License-Identifier: MIT

pragma solidity 0.8.28;

import { Queue } from "../../contracts/bridge/libraries/Queue.sol";

contract QueueTest {
    Queue.QueueData messageQueue;

    function getSize() external view returns (uint256) {
        return Queue.getSize(messageQueue);
    }

    function isEmpty() external view returns (bool) {
        return Queue.isEmpty(messageQueue);
    }

    function pushBack(bytes32 messageHash) external {
        return Queue.pushBack(messageQueue, messageHash);
    }

    function front() external view returns (bytes32) {
        return Queue.front(messageQueue);
    }

    function popFront() external returns (bytes32 messageHash) {
        return Queue.popFront(messageQueue);
    }

    function popFrontBatch(uint256 count) external returns (bytes32[] memory messageHashes) {
        return Queue.popFrontBatch(messageQueue, count);
    }
}
