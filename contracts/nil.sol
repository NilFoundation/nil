// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

library nil {

    function async_call(address dst, uint gas, bool deploy, uint value, bytes memory call_data) internal {
        bytes memory data = abi.encode(deploy, dst, gas, call_data);
        bool success;

        bytes memory returnData;
        (success, returnData) = address(0xfd).call{value: value, gas: gasleft()}(data);

        require(success, "Precompiled contract call failed");
    }

    // Send raw internal message using a special precompiled contract
    function send_msg(uint g, bytes memory message) internal {
        uint message_size = message.length;
        assembly {
            // Call precompiled contract.
            // Arguments: gas, precompiled address, value, input, input size, output, output size
            if iszero(call(g, 0xfc, 0, add(message, 32), message_size, 0, 0)) {
                revert(0, 0)
            }
        }
    }

    // Function to call the validateSignature precompiled contract
    function validateSignature(bytes memory pubkey, uint256 hash, bytes memory signature) internal view returns (bool) {
        // ABI encode the input parameters
        bytes memory encodedInput = abi.encode(pubkey, hash, signature);
        bool success;
        bool result;

        // Perform the static call to the precompiled contract at address 0xfe
        bytes memory returnData;
        (success, returnData) = address(0xfe).staticcall(encodedInput);

        require(success, "Precompiled contract call failed");

        // Extract the boolean result from the returned data
        if (returnData.length > 0) {
            result = abi.decode(returnData, (bool));
        }

        return result;
    }
}
