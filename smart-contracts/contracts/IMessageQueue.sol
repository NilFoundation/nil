// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

interface IMessageQueue {
    function sendRawTransaction(bytes calldata _message) external;
}
