// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "./NilTokenBase.sol";

/**
 * @title SmartAccount
 * @dev Basic Smart Account contract that provides functionality for interacting
 * with other contracts and sending tokens.  It also supports multi-token
 * functionality, including methods for minting and sending tokens.
 * The NilTokenBase class implements functionality for managing the contract's own
 * token (where `tokenId = address(this)`).
 */
contract SmartAccount is NilTokenBase {
    bytes pubkey;

    /**
     * @dev Fallback function to receive Ether.
     */
    receive() external payable {}

    /**
     * @dev Function to handle bounce transactions.
     * @param err The error transaction.
     */
    function bounce(string calldata err) external payable {}

    /**
     * @dev Constructor to initialize the smart account with a public key.
     * @param _pubkey The public key to initialize the smart account with.
     */
    constructor(bytes memory _pubkey) payable {
        pubkey = _pubkey;
    }

    /**
     * @dev Sends raw transaction.
     * @param transaction The raw transaction to send.
     */
    function send(bytes calldata transaction) public onlyExternal {
        Nil.sendTransaction(transaction);
    }

    /**
     * @dev Deploys a contract asynchronously.
     * @param shardId The shard ID where to deploy contract.
     * @param value The value to send.
     * @param code The init code to be deployed. Constructor arguments must be appended to it.
     * @param salt Salt for the contract address creation.
     */
    function asyncDeploy(
        uint shardId,
        uint value,
        bytes calldata code,
        uint salt
    ) public onlyExternal {
        Nil.asyncDeploy(shardId, address(this), value, code, salt);
    }

    /**

     * @dev Makes an asynchronous call.
     * @param dst The destination address.
     * @param refundTo The address where to send refund transaction.
     * @param bounceTo The address where to send bounce transaction.
     * @param tokens Multi-tokens to send.
     * @param value The value to send.
     * @param callData The call data of the called method.
     */
    function asyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        Nil.Token[] memory tokens,
        uint value,
        bytes calldata callData
    ) public onlyExternal {
        Nil.asyncCallWithTokens(
            dst,
            refundTo,
            bounceTo,
            0,
            Nil.FORWARD_REMAINING,
            value,
            tokens,
            callData
        );
    }

    /**
     * @dev Makes a synchronous call, which is just a regular EVM call, without using transactions.
     * @param dst The destination address.
     * @param feeCredit The amount of tokens available to pay all fees during transaction processing.
     * @param value The value to send.
     * @param call_data The call data of the called method.
     */
    function syncCall(
        address dst,
        uint feeCredit,
        uint value,
        bytes memory call_data
    ) public onlyExternal {
        (bool success, ) = dst.call{value: value, gas: feeCredit}(call_data);
        require(success, "Call failed");
    }

    /**
     * @dev Verifies an external transaction.
     * @param hash The hash of the data.
     * @param signature The signature to verify.
     * @return True if the signature is valid, false otherwise.
     */
    function verifyExternal(
        uint256 hash,
        bytes calldata signature
    ) external view returns (bool) {
        return Nil.validateSignature(pubkey, hash, signature);
    }
}
