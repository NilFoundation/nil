// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.0;

import "../lib/NilTokenBase.sol";

contract Token is NilTokenBase {

    constructor() payable {
        mintTokenInternal(10000000000);
        sendTokenInternal(msg.sender, getTokenId(), 10000000000);
    }

    receive() external payable {}

    function verifyExternal(uint256 hash, bytes calldata signature) external view returns (bool) {
        return true;
    }

}
