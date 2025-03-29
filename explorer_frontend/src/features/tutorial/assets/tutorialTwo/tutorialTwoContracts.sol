pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/**
 * @title Operator
 * @author =nil; Foundation
 * @notice A contract for performing operations on CustomToken.
 * Should not be modified.
 */
contract Operator is NilBase {
    /**
     * The default function for receiving calls with empty call data.
     */
    receive() external payable {}

    /**
     * The function calling mintTokenCustom() on CustomToken.
     * @param dst The address of CustomToken.
     * @param amount The amount of the custom token to mint.
     */
    function checkMintToken(address dst, uint256 amount) public payable {
        Nil.asyncCall(
            dst,
            address(0),
            0,
            abi.encodeWithSignature("mintTokenCustom(uint256)", amount)
        );
    }

    /**
     * The function calling sendTokenCustom() on CustomToken.
     * @param dst The address of CustomToken.
     * @param amount The amount of the custom token to send.
     */
    function checkSendToken(address dst, uint256 amount) public payable {
        Nil.asyncCall(
            dst,
            address(0),
            0,
            abi.encodeWithSignature("sendTokenCustom(uint256)", amount)
        );
    }
}

/**
 * @title CustomToken
 * @author =nil; Foundation
 * @notice A contract representing a custom token.
 */
contract CustomToken is NilTokenBase {
    address operatorAddress;

    /**
     * The constructor must set operatorAddress to _operatorAddress.
     * @param _operatorAddress The address of the Operator contract.
     */
    constructor(address _operatorAddress) {}

    /**
     * A custom wrapper for mintTokenInternal().
     * @param amount The amount of the custom token to mint.
     */
    function mintTokenCustom(uint256 amount) public payable {}

    /**
     * A custom wrapper for sendTokenInternal().
     * @param amount The amount of the custom token to send.
     */
    function sendTokenCustom(uint256 amount) public payable {}
}
