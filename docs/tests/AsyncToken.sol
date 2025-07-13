// SPDX-License-Identifier: MIT
pragma solidity ^0.8.21;

//startAsyncTokenContract

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract AsyncTokenSender is NilBase {
    function sendTokenAsync(uint amount, address dst) public async(2_000_000) {
        Nil.Token[] memory tokens = Nil.txnTokens();
        Nil.asyncCallWithTokens(
            dst,
            Nil.msgSender(),
            address(this),
            0,
            Nil.FORWARD_REMAINING,
            0,
            tokens,
            "",
            0,
            0
        );
    }
}

//endAsyncTokenContract
