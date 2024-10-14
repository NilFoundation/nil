// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/NilCurrencyBase.sol";

contract Currency is NilCurrencyBase {
    constructor(uint256 initialSupply) {
        // Mint the initial supply of tokens
        mintCurrencyInternal(initialSupply);
    }

    // Public function to call the parent internal function sendCurrencyInternal
    function transferCurrency(address to, CurrencyId currencyId, uint256 amount) public {
        sendCurrencyInternal(to, currencyId, amount);
    }
}
