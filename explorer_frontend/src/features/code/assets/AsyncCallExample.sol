// SPDX-License-Identifier: MIT

pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

// Caller contract is a simple delegate proxy
// It demonstrates how to interact with another contract (Counter)
// Located (possibly) on a different shard
// To test, deploy Caller and Counter on separate shards
// The call method uses Nil.asyncCall to send an asynchronous call to the Counter contract
// Async call arguments: destination address (dst), callback address (msg.sender),
// value (0 in this example), and encoded function signature (increment())

// read more:
// https://docs.nil.foundation/nil/key-principles/async-execution
// https://docs.nil.foundation/nil/smart-contracts/handling-async-execution/

contract Caller {
    using Nil for address;

    // Sends an async call to the Counter contract to invoke the increment method.
    // dst: address of the Counter contract
    function call(address dst) public {
        Nil.asyncCall(
            dst, // Address of the destination contract (Counter)
            msg.sender, // Bounce address
            0, // Value to send with the call
            abi.encodeWithSignature("increment()") // Encoded signature of the increment function
        );
    }
}

// Counter contract is a simple stateful contract that keeps track of a counter value
// It provides methods to increment the value and read the current value

contract Counter {
    uint256 private value; // Stores the current counter value

    // Increments the counter by 1
    function increment() public {
        value += 1;
    }

    // Returns the current value of the counter
    function getValue() public view returns (uint256) {
        return value;
    }
}
