// SPDX-License-Identifier: MIT

pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

// Here we create a simple custom token whose maximal circulating amount
// Is equal to some creator-defined number
// Anybody can mint new instance of the token
// But if limit is reached mint will finish with error
// Freshly minted tokens are immediately sent to the minter

// To test deploy the `CappedToken` contract
// Then call `mintTokenPayableWrapper` with 0.003 attached value and amount equal to 1

// https://docs.nil.foundation/nil/key-principles/tokens
// https://docs.nil.foundation/nil/smart-contracts/tokens
// https://docs.nil.foundation/nil/smart-contracts/func-modifiers/#onlyinternal-and-onlyexternal

contract CappedToken is NilTokenBase {
    uint256 cap;

    // set limit for amount of tokens minted
    constructor(uint256 _cap) {
        cap = _cap;
    }

    // base contract `mintToken` is nonpayable and `onlyExternal`
    // cause it's supposed to be called with external transactions
    // (which literally cannot carry any value)
    // thus we create an additional wrapper method here to add a payable functionality
    function mintTokenPayableWrapper(uint256 amount) public payable {
      // price of each token is 0.003
      require(msg.value * 1000 == amount * 3, "not enough funds to mint tokens");

      mintToken(amount);
    }

    function mintToken(uint256 amount) public override {
        require(totalSupply + amount <= cap, "cannot mint that many tokens");

        mintTokenInternal(amount);

        // send minted tokens to the transaction initiator
        sendToken(msg.sender, TokenId.wrap(address(this)), amount);
    }
}
