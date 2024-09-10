// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../lib/Nil.sol";

// Bug was in the `STATICCALL` opcode, it was executed as a `DELEGATECALL`. Thus, `value = 42` isn't read via getValue().

contract StaticCallQuery {
    function checkValue(address source, uint256 value) view external {
        uint256 getValue = StaticCallSource(source).getValue();
        require(getValue == value, "Value is not correct");
    }

    // Try to change state of StaticCallSource contract via `Nil.syncCall`.
    function querySyncIncrement(address source) external {
        Nil.Token[] memory tokens;
        bytes memory data = abi.encodeWithSignature("increment()");
        (bool success, ) = Nil.syncCall(source, gasleft(), 0, tokens, data);
        require(success, "Call failed");
    }
}

contract StaticCallSource {
    uint256 private value = 42;

    function getValue() external view returns (uint256) {
        return value;
    }

    function increment() external {
        value++;
    }
}
