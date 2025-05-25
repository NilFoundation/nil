// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Token is NilTokenBase {

    constructor(string memory _tokenName, uint256 initialSupply) {
        tokenName = _tokenName;
        mintTokenInternal(initialSupply);
    }

    function mintTokenPublic(uint256 amount) public {
        mintTokenInternal(amount);
    }

    function sendTokenPublic(uint256 shardIdDst, address to, TokenId tokenId, uint256 amount) public {
        sendTokenInternal(shardIdDst, to, tokenId, amount);
    }

    receive() external payable {}
}
