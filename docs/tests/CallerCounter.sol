// SPDX-License-Identifier: MIT

pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Caller {
    using Nil for address;

    receive() external payable {}

    function call(address dst) public {
        Nil.asyncCall(
            dst,
            msg.sender,
            0,
            abi.encodeWithSignature("increment()")
        );
    }

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
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
