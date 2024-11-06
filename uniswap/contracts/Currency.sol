// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/NilCurrencyBase.sol";

contract Currency is NilCurrencyBase {
    bytes pubkey;

    constructor(string memory _currencyName, bytes memory _pubkey) payable {
        // Revert if the currency name is an empty string
        require(bytes(_currencyName).length > 0, "Currency name must not be empty");
        pubkey = _pubkey;
        tokenName = _currencyName;
    }
    receive() external payable {}

    function verifyExternal(uint256 hash, bytes calldata signature) external view returns (bool) {
        return Nil.validateSignature(pubkey, hash, signature);
    }
}