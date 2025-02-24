// SPDX-License-Identifier: MIT
//startContract
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/**
 * @title FT
 * @author =nil; Foundation
 * @notice The contract represents a simple fungible token.
 */
contract FT is NilTokenBase {
    /**
     * @notice A public 'wrapper' over mintTokenInternal().
     */
    function mintFT(uint256 amount) public {
        mintTokenInternal(amount);
    }

    /**
     * @notice The function sends the FT to the provided address.
     * @param dst The address to which the FT must be sent.
     */
    function sendFT(address dst, uint256 amount) public {
        uint currentBalance = getTokenTotalSupply();
        require(amount <= currentBalance, "Insufficient balance");
        Nil.Token[] memory ft = new Nil.Token[](1);
        ft[0].id = getTokenId();
        ft[0].amount = amount;
        Nil.asyncCallWithTokens(
            dst,
            msg.sender,
            msg.sender,
            0,
            Nil.FORWARD_REMAINING,
            0,
            ft,
            ""
        );
    }

    /**
     *
     * @notice The empty override ensures that the FT can only be minted via mintFT().
     */
    function mintToken(uint256 amount) public override onlyExternal {}
}
//endContract
