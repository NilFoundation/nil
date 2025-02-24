// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

contract Oracle is NilBase {
    mapping(TokenId => uint256) public rates;

    function setPrice(TokenId token, uint256 price) public {
        rates[token] = price;
    }

    function getPrice(TokenId token) public view returns (uint256){
        return rates[token];
    }
 }