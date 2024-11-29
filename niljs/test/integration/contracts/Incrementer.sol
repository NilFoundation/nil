// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Incrementer {
    uint256 public counter;
    constructor(uint256 start) {
        counter = start;
    }

    function increment() public {
        counter += 1;
    }

    function getCounter() public view returns (uint256) {
        return counter;
    }

    function setCounter(uint256 _counter) public {
        counter = _counter;
    }

    function add(uint256 a, uint256 b) public pure returns (uint256) {
        return a + b;
    }
}
