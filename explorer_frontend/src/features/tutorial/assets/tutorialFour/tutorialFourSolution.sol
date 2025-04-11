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
    address private owner;

    constructor() public payable {
        owner = msg.sender;
    }

    function deposit() public payable {}

    modifier onlyOwner() {
        require(msg.sender == owner, "Only owner can call this function");
        _;
    }

    /**
     * The function for minting the NFT.
     * It must call mintTokenInternal().
     */
    function mintNFT() public {
        require(totalSupply == 0, "NFT has already been minted");
        require(!hasBeenSent, "NFT has already been sent");
        mintTokenInternal(1);
    }

    /**
     * The function for sending the NFT.
     * It must call sendTokenInternal().
     * @param to The address where the NFT should be sent.
     */
    function sendNFT(address to) public {
        require(!hasBeenSent, "NFT has already been sent");
        Nil.Token[] memory nft = new Nil.Token[](1);
        nft[0].id = getTokenId();
        nft[0].amount = 1;
        Nil.asyncCallWithTokens(
            to,
            msg.sender,
            msg.sender,
            0,
            Nil.FORWARD_REMAINING,
            0,
            nft,
            abi.encodeWithSignature("deposit()")
        );
        hasBeenSent = true;
    }
}
