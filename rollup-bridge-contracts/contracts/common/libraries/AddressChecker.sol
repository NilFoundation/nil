// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;
/**
 * @dev Library for Address dataType
 *
 */

library AddressChecker {
  /**
   * @dev Returns true if `account` is a contract.
   *
   * [IMPORTANT]
   * ====
   * It is unsafe to assume that an address for which this function returns
   * false is an externally-owned account (EOA) and not a contract.
   *
   * Among others, `isContract` will return false for the following
   * types of addresses:
   *
   *  - an externally-owned account
   *  - a contract in construction
   *  - an address where a contract will be created
   *  - an address where a contract lived, but was destroyed
   *
   * Furthermore, `isContract` will also return true if the target contract within
   * the same transaction is already scheduled for destruction by `SELFDESTRUCT`,
   * which only has an effect at the end of a transaction.
   * ====
   *
   * [IMPORTANT]
   * ====
   * You shouldn't rely on `isContract` to protect against flash loan attacks!
   *
   * Preventing calls from contracts is highly discouraged. It breaks composability, breaks support for smart wallets
   * like Gnosis Safe, and does not provide security since it can be circumvented by calling from a contract
   * constructor.
   * ====
   */
  function isContract(address account) internal view returns (bool) {
    // This method relies on extcodesize/address.code.length, which returns 0
    // for contracts in construction, since the code is only stored at the end
    // of the constructor execution.
    return account != address(0) && account.code.length > 0;
  }

  function isAValidAddress(address account) internal view returns (bool) {
    return account != address(0);
  }
}
