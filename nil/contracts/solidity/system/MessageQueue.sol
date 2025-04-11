// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import "../lib/IMessageQueue.sol";

struct Message {
    bytes data;
    address sender;
}

contract MessageQueue is IMessageQueue{
    function sendRawTransaction(bytes calldata _message) external override {
        queue.push(Message({
            data: _message,
            sender: msg.sender
        }));
    }

    function getMessages() external view returns (Message[] memory) {
        return queue;
    }

    function clearQueue() external {
        require(msg.sender == address(this), "clearQueue: only MessageQueue contract can be caller of this function");
        delete queue;
    }

    Message[] private queue;
}
