// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

library Nil {
    uint private constant SEND_MESSAGE = 0xfc;
    address private constant ASYNC_CALL = address(0xfd);
    address public constant VERIFY_SIGNATURE = address(0xfe);
    address public constant IS_INTERNAL_MESSAGE = address(0xff);

    function asyncCall(
        address dst,
        address refundTo,
        uint gas,
        bool deploy,
        uint value,
        bytes memory callData
    ) internal {
        bytes memory data = abi.encode(deploy, dst, refundTo, gas, callData);
        bool success;

        bytes memory returnData;
        (success, returnData) = ASYNC_CALL.call{value: value, gas: gasleft()}(
            data
        );

        require(success, "Precompiled contract call failed");
    }

    // Send raw internal message using a special precompiled contract
    function sendMessage(uint g, bytes memory message) internal {
        uint message_size = message.length;
        assembly {
            // Call precompiled contract.
            // Arguments: gas, precompiled address, value, input, input size, output, output size
            if iszero(
                call(g, SEND_MESSAGE, 0, add(message, 32), message_size, 0, 0)
            ) {
                revert(0, 0)
            }
        }
    }

    // Function to call the validateSignature precompiled contract
    function validateSignature(
        bytes memory pubkey,
        uint256 hash,
        bytes memory signature
    ) internal view returns (bool) {
        // ABI encode the input parameters
        bytes memory encodedInput = abi.encode(pubkey, hash, signature);
        bool success;
        bool result;

        // Perform the static call to the precompiled contract at address `VerifyExternalMessage`
        bytes memory returnData;
        (success, returnData) = VERIFY_SIGNATURE.staticcall(encodedInput);

        require(success, "Precompiled contract call failed");

        // Extract the boolean result from the returned data
        if (returnData.length > 0) {
            result = abi.decode(returnData, (bool));
        }

        return result;
    }
}

contract NilBase {
    // Check that method was invoked from internal message
    modifier onlyInternal() {
        require(
            isInternalMessage(),
            "Try to call internal function with external message"
        );
        _;
    }

    // Check that method was invoked from external message
    modifier onlyExternal() {
        require(
            !isInternalMessage(),
            "Try to call external function with internal message"
        );
        _;
    }

    function isInternalMessage() internal view returns (bool) {
        bytes memory data;
        (bool success, bytes memory returnData) = Nil
            .IS_INTERNAL_MESSAGE
            .staticcall(data);
        require(success, "Precompiled contract call failed");
        require(
            returnData.length > 0,
            "'IS_INTERNAL_MESSAGE' returns invalid data"
        );
        return abi.decode(returnData, (bool));
    }
}
