// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

 contract GlobalLedger {
     mapping(address => mapping(TokenId => uint256)) public deposits;
     mapping(address => Loan) public loans;
 
     struct Loan {
         uint256 amount;
         TokenId token;
     }
 
     function recordDeposit(address user, TokenId token, uint256 amount) public {
         deposits[user][token] += amount;
     }
 
     function getDeposit(address user, TokenId token) public view returns (uint256) {
         return deposits[user][token];
     }
 
     function recordLoan(address user, TokenId token, uint256 amount) public {
         loans[user] = Loan(amount, token);
     }
 
     function getLoanDetails(address user) public view returns (uint256, TokenId) {
     return (loans[user].amount, loans[user].token);
 }
 
 }