// SPDX-License-Identifier: MIT

pragma solidity ^0.8.21;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/**
 * @title Receiver
 * @author =nil; Foundation
 * @notice A simple contract for storing the NFT.
 */
contract Receiver {
    function deposit() public payable {}
}

/**
 * @title NFT
 * @author =nil; Foundation
 * @notice A contract representing a non-fungible token.
 */
contract NFT is NilTokenBase {
    bool private hasBeenSent = false;

    function deposit() public payable {}

    /**
     * The function for minting the NFT.
     * It must call mintTokenInternal().
     * It must also be protected against repeated minting.
     * Hint: use totalSupply to eliminate repeated minting.
     */
    function mintNFT() public payable {
        // TODO: complete the function
    }

    /**
     * The function for sending the NFT.
     * It must call sendTokenInternal().
     * @param to The address where the NFT should be sent.
     * Hint: call the deposit() function inside Receiver.
     */
    function sendNFT(address to) public {
        // TODO: complete the function
    }
}
