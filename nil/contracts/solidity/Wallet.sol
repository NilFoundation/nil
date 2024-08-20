// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "./NilCurrencyBase.sol";

/**
 * @title Wallet
 * @dev Basic Wallet contract which provides functional for calling another contracts and sending tokens.
 * It also supports multi-currency functionality providing methods for minting and sending currency.
 * NilCurrencyBase class implements functional for managing own currency(where `currencyId = address(this)`).
 */
contract Wallet is NilCurrencyBase {

    bytes pubkey;

    /**
     * @dev Fallback function to receive Ether.
     */
    receive() external payable {}

    /**
     * @dev Function to handle bounce messages.
     * @param err The error message.
     */
    function bounce(string calldata err) external payable {}

    /**
     * @dev Constructor to initialize the wallet with a public key.
     * @param _pubkey The public key to initialize the wallet with.
     */
    constructor(bytes memory _pubkey) payable {
        pubkey = _pubkey;
    }

    /**
     * @dev Sends raw message.
     * @param message The raw message to send.
     */
    function send(bytes calldata message) onlyExternal public {
        Nil.sendMessage(gasleft(), message);
    }

    /**
     * @dev Makes an asynchronous call.
     * @param dst The destination address.
     * @param refundTo The address where to send refund message.
     * @param bounceTo The address where to send bounce message.
     * @param feeCredit The amount of tokens available to pay all fees during message processing.
     * @param deploy Whether to deploy the contract.
     * @param tokens The multi-currency tokens to send.
     * @param value The value to send.
     * @param callData The call data of the called method.
     */
    function asyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        bool deploy,
        Nil.Token[] memory tokens,
        uint value,
        bytes calldata callData) onlyExternal public {
        Nil.asyncCall(dst, refundTo, bounceTo, feeCredit, Nil.FORWARD_NONE, deploy, value, tokens, callData);
    }

    /**
     * @dev Makes a synchronous call, which is just a regular EVM call, without using messages.
     * @param dst The destination address.
     * @param feeCredit The amount of tokens available to pay all fees during message processing.
     * @param value The value to send.
     * @param call_data The call data of the called method.
     */
    function syncCall(address dst, uint feeCredit, uint value, bytes memory call_data) onlyExternal public {
        (bool success,) = dst.call{value: value, gas: feeCredit}(call_data);
        require(success, "Call failed");
    }

    /**
     * @dev Verifies an external message.
     * @param hash The hash of the data.
     * @param signature The signature to verify.
     * @return True if the signature is valid, false otherwise.
     */
    function verifyExternal(uint256 hash, bytes calldata signature) external view returns (bool) {
        return Nil.validateSignature(pubkey, hash, signature);
    }
}
