// SPDX-License-Identifier: MIT
pragma solidity ^0.8.21;

//startAsyncCurrencyContract

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract AsyncCurrencySender {
  function sendCurrencyAsync(uint amount, address dst) public {
    Nil.Token[] memory tokens = Nil.msgTokens();
    Nil.asyncCallWithTokens(
      dst,
      msg.sender,
      address(this),
      0,
      Nil.FORWARD_REMAINING,
      false,
      0,
      tokens,
      ""
    );
  }
}

//endAsyncCurrencyContract