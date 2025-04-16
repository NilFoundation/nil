// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

contract Token is NilTokenBase {

    constructor(string memory _tokenName, uint256 initialSupply) payable {
        tokenName = _tokenName;
        mintTokenInternal(initialSupply);
    }

    receive() external payable {}
}
