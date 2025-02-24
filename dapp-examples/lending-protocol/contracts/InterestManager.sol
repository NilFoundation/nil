// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

contract InterestManager {

    function getInterestRate() public pure returns (uint256) {
        return 5;                  
    }
}
