// SPDX-License-Identifier: MIT

pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

// Caller contract is a simple proxy
// It demonstrates how to interact with another contract (Counter)
// Located (possibly) on a different shard
// In this case delivery of either response or error is guaranteed by the system

// To test, deploy Caller and Counter on separate shards
// Try executing `Caller.call` 5 times

// read more:
// https://docs.nil.foundation/nil/key-principles/async-execution
// https://docs.nil.foundation/nil/smart-contracts/handling-async-execution/#retreiving-values

contract Caller {
    using Nil for address;

    uint256 public result;

    function callback(
        bool success,
        bytes memory returnData,
        bytes memory
    ) public {
        require(success == true, "Result not true");
        result = abi.decode(returnData, (uint256));
    }

    // Sends an async request to the Counter contract to invoke the increment method
    // It's guaranteed by the system that either response or error will be returned
    function call(address dst) public {
        bytes memory context = abi.encodeWithSelector(this.callback.selector);
        Nil.sendRequest(
            dst, // Address of the destination contract (Counter)
            0, // Amount of value to send
            Nil.ASYNC_REQUEST_MIN_GAS, // Amount of gas reserved to process the response
            context, // Context for the callback function
            abi.encodeWithSignature("increment()") // Encoded signature of the increment function
        );
    }
}

// Counter contract is a simple stateful contract that keeps track of a counter value
// It provides method to increment the value

contract Counter {
    uint256 private value; // Stores the current counter value

    // Increments the counter by 1 and returns its value
    function increment() public returns (uint256){
        value += 1;

        // dummy condition to illustrate error response after several calls
        require(value < 5, "Limit reached");

        return value;
    }
}
