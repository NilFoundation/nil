// SPDX-License-Identifier: MIT

pragma solidity ^0.8.9;

contract Caller {
    function sendValue(address dst) public {}
}

contract Receiver {
    function depositAndReturn() public {}
}