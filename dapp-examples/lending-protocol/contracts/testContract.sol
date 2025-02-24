// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

contract LendingPool is NilBase, NilTokenBase {
    address public globalLedger;
    TokenId public usdt;
    TokenId public eth;

    constructor(
        address _globalLedger,
        TokenId _usdt,
        TokenId _eth
    ) {
        globalLedger = _globalLedger;
        usdt = _usdt;
        eth = _eth;
    }

    function deposit(bytes memory callData) public payable {
        // Nil.Token[] memory tokens = Nil.txnTokens();
        // require(tokens.length == 1, "Only one token at a time");
        // require(tokens[0].id == usdt || tokens[0].id == eth, "Invalid token");
        // uint256 tokenAmount = tokens[0].amount;
        // bytes memory callData = "0x1e578bae00000000000000000000000000011220c56e7fc557b98bbec6542c174b3a298200000000000000000000000000000000000000000000003635c9adc5dea00000";
          Nil.asyncCall(globalLedger, address(this), 0, callData);
    }

}
contract GlobalLedger {
    mapping(address => uint256) public deposits;

    function recordDeposit(address user, uint256 amount) public {
        deposits[user] += amount;
    }

}
