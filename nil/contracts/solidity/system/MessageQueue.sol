// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import "./IMessageQueue.sol";

contract MessageQueue is IMessageQueue{
    constructor(uint _nShards) {
        nShards = _nShards;
    }

    function sendMessage(
        uint _shardId,
        bytes calldata _message
    ) external override {
        require(_shardId != 0, "sendMessage: shardId must not be 0");
        require(_shardId < nShards, "sendMessage: shardId must be less than nShards");
        queues[_shardId].push(_message);
    }

    function getMessages(
        uint _shardId
    ) external view returns (bytes[] memory) {
        return queues[_shardId];
    }

    function clearQueues() external {
        require(msg.sender == address(this), "clearQueues: only MessageQueue contract can be caller of this function");
        for (uint i = 0; i < nShards; i++) {
            delete queues[i];
        }
    }

    function updateNShards(uint _nShards) external {
        require(msg.sender == address(this), "clearQueues: only MessageQueue contract can be caller of this function");
        nShards = _nShards;
    }

    mapping(uint => bytes[]) private queues;
    uint private nShards;
}
