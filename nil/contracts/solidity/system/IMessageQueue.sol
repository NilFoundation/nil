// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

interface IMessageQueue {
    function sendMessage(uint _shardId, bytes calldata _message) external;
}
