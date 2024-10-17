// SPDX-License-Identifier: MIT
pragma solidity ^0.8.11;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Awaiter {
    using Nil for address;

    uint256 public result;

    function call(address dst) public {
        bytes memory temp;
        bool ok;
        (temp, ok) = Nil.awaitCall(
            dst,
            Nil.ASYNC_REQUEST_MIN_GAS,
            abi.encodeWithSignature("getValue()")
        );

        require(ok == true, "Result not true");

        result = abi.decode(temp, (uint256));
    }

    function getResult() public view returns (uint256) {
        return result;
    }
}

contract Counter {
    uint256 private value;

    event ValueChanged(uint256 newValue);

    function increment() public {
        value += 1;
        emit ValueChanged(value);
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}
