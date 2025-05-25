// SPDX-License-Identifier: MIT
pragma solidity ^0.8.21;

//startAsyncTokenContract

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract AsyncTokenSender {
    function sendTokenAsync(uint256 shardIdDst, uint amount, address dst) public {
        Nil.Token[] memory tokens = Nil.txnTokens();
        Nil.asyncCallWithTokens(
            shardIdDst,
            dst,
            msg.sender,
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
