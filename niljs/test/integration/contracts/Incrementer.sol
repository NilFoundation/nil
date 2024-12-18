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

    function incrementExternal() public onlyExternal {
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

    receive() external payable {}

    function verifyExternal(uint256 hash, bytes calldata signature) external view returns (bool) {
        return true;
    }

    modifier onlyExternal() {
        require(!isInternalMessage(), "Trying to call external function with internal message");
        _;
    }

    // isInternalMessage returns true if the current message is internal.
    function isInternalMessage() internal view returns (bool) {
        bytes memory data;
        (bool success, bytes memory returnData) = address(0xff).staticcall(data);
        require(success, "Precompiled contract call failed");
        require(returnData.length > 0, "'IS_INTERNAL_MESSAGE' returns invalid data");
        return abi.decode(returnData, (bool));
    }
}
