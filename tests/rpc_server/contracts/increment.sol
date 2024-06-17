pragma solidity ^0.8.0;

contract Incrementer {
    uint256 private value;

    function increment() public {
        value += 1;
    }

    function get() public view returns(uint256) {
        return value;
    }
}
