// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

// https://docs.nil.foundation/nil/key-principles/tokens
// https://docs.nil.foundation/nil/smart-contracts/tokens
// https://docs.nil.foundation/nil/smart-contracts/func-modifiers/#onlyinternal-and-onlyexternal
contract NilEnshrinedToken is NilTokenBase, Ownable {
  error ErrorInvalidAmount();

  // set name of the token
  constructor(string memory _tokenName) Ownable(msg.sender) {
    setTokenName(_tokenName);
  }

  function mintToken(uint256 amount) public override onlyOwner {
    if (amount == 0) {
      revert ErrorInvalidAmount();
    }

    mintTokenInternal(amount);

    // send minted tokens to the transaction initiator
    sendToken(msg.sender, TokenId.wrap(address(this)), amount);
  }
}
