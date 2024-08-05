// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

library Nil {
    uint private constant SEND_MESSAGE = 0xfc;
    address private constant ASYNC_CALL = address(0xfd);
    address public constant VERIFY_SIGNATURE = address(0xfe);
    address public constant IS_INTERNAL_MESSAGE = address(0xff);
    address private constant MINT_CURRENCY = address(0xd0);
    address private constant GET_CURRENCY_BALANCE = address(0xd1);
    address private constant SEND_CURRENCY_SYNC = address(0xd2);
    address private constant GET_MESSAGE_TOKENS = address(0xd3);

    address payable public constant MINTER_ADDRESS = payable(address(0x0001222222222222222222222222222222222222));

    // Token is a struct that represents a token with an id and amount.
    struct Token {
        uint256 id;
        uint256 amount;
    }

    // asyncCall is a function that makes an asynchronous call to `dst` contract.
    function asyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        uint gas,
        bool deploy,
        uint value,
        bytes memory callData
    ) internal returns(bool) {
        Token[] memory tokens;
        return asyncCall(dst, refundTo, bounceTo, gas, deploy, value, tokens, callData);
    }

    // asyncCall is a function that makes an asynchronous call to `dst` contract.
    // This function is used to call a contract with a list of tokens.
    function asyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        uint gas,
        bool deploy,
        uint value,
        Token[] memory tokens,
        bytes memory callData
    ) internal returns(bool) {
        bool success = Precompile(ASYNC_CALL).precompileAsyncCall{value: value}(deploy, dst, refundTo, bounceTo, gas,
            tokens, callData);
        return success;
    }

    function syncCall(
        address dst,
        uint gas,
        uint value,
        Token[] memory tokens,
        bytes memory callData
    ) internal returns(bool, bytes memory) {
        if (tokens.length > 0) {
            Precompile(SEND_CURRENCY_SYNC).precompileSendTokens(dst, tokens);
        }
        (bool success, bytes memory returnData) = dst.call{gas: gas, value: value}(callData);
        return (success, returnData);
    }

    // Send raw internal message using a special precompiled contract
    function sendMessage(uint g, bytes memory message) internal {
        uint message_size = message.length;
        assembly {
        // Call precompiled contract.
        // Arguments: gas, precompiled address, value, input, input size, output, output size
            if iszero(call(g, SEND_MESSAGE, 0, add(message, 32), message_size, 0, 0)) {
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

    // mintCurrency mints a token with a given id and amount. Can be called only by the special minter contract.
    // Returns `true` if the minting was successful.
    function mintToken(uint256 id, uint256 amount) internal returns(bool) {
        return Precompile(MINT_CURRENCY).precompileMintCurrency(id, amount);
    }

    // getCurrencyBalance returns the balance of a token with a given id for a given address.
    function tokensBalance(address addr, uint256 id) internal returns(uint256) {
        return Precompile(GET_CURRENCY_BALANCE).precompileGetCurrencyBalance(id, addr);
    }

    // msgTokens returns tokens from the current message.
    function msgTokens() internal returns(Token[] memory) {
        return Precompile(GET_MESSAGE_TOKENS).precompileGetMessageTokens();
    }
}

// NilBase is a base contract that provides modifiers for checking the type of message (internal or external).
contract NilBase {
    // Check that method was invoked from internal message
    modifier onlyInternal() {
        require(isInternalMessage(), "Trying to call internal function with external message");
        _;
    }

    // Check that method was invoked from external message
    modifier onlyExternal() {
        require(!isInternalMessage(), "Trying to call external function with internal message");
        _;
    }

    function isInternalMessage() internal view returns (bool) {
        bytes memory data;
        (bool success, bytes memory returnData) = Nil.IS_INTERNAL_MESSAGE.staticcall(data);
        require(success, "Precompiled contract call failed");
        require(returnData.length > 0, "'IS_INTERNAL_MESSAGE' returns invalid data");
        return abi.decode(returnData, (bool));
    }
}

// Precompile is a contract that provides stubs for precompiled contract calls.
// NOTE: Function should always return value, otherwise Solidity will check contract existence by EXTCODESIZE opcode
contract Precompile {
    function precompileMintCurrency(uint256 id, uint256 amount) public returns(bool) {}
    function precompileGetCurrencyBalance(uint256 id, address addr) public returns(uint256) {}
    function precompileAsyncCall(bool, address, address, address, uint, Nil.Token[] memory, bytes memory) public payable returns(bool) {}
    function precompileSendTokens(address, Nil.Token[] memory) public returns(bool) {}
    function precompileGetMessageTokens() public returns(Nil.Token[] memory) {}
}

abstract contract NilBounceable is NilBase {
    function bounce(string calldata err) virtual payable external;
}