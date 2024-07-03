// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

// Common test contract. Can be used in any test.
contract Test {

    function getSum(uint a, uint b) public payable returns(uint) {
        return a + b;
    }

    function getString() public payable returns(string memory) {
        return "Very long string with many characters and words and spaces and numbers and symbols and everything else that can be in a string";
    }

    function getNumAndString() public payable returns(uint, string memory) {
        return (123456789012345678901234567890, "Simple string");
    }

    function noReturn() public payable {}
}
