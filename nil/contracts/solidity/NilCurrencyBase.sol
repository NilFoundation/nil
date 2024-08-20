// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Nil.sol";

/**
 * @title NilCurrencyBase
 * @dev Abstract contract that provides functionality for currency processing.
 * Methods with "Internal" suffix are internal, which means that they can be called only from the derived contract
 * itself. But there are default wrapper methods that provide the account owner access to internal methods.
 * They are virtual, so the main contract can disable them by overriding them. Then only logic of the contract can use
 * internal methods.
 */
abstract contract NilCurrencyBase is NilBase {
    uint totalSupply;
    string tokenName;

    /**
     * @dev Returns the total supply of the currency.
     * @return The total supply of the currency.
     */
    function getCurrencyTotalSupply() public view returns(uint) {
        return totalSupply;
    }

    /**
     * @dev Returns the balance of the currency owned by this contract.
     * @return The balance of the currency owned by this contract.
     */
    function getOwnCurrencyBalance() public view returns(uint256) {
        return Nil.currencyBalance(address(this), getCurrencyId());
    }

    /**
     * @dev Returns the unique identifier of the currency owned by this contract.
     * @return The unique identifier of the currency owned by this contract.
     */
    function getCurrencyId() public view returns(uint256) {
        return uint256(uint160(address(this)));
    }

    /**
     * @dev Returns the name of the currency.
     * @return The name of the currency.
     */
    function getCurrencyName() public view returns(string memory) {
        return tokenName;
    }

    /**
     * @dev Set the name of the currency.
     * @param name The name of the currency.
     */
    function setCurrencyName(string memory name) onlyExternal virtual public {
        tokenName = name;
    }

    /**
     * @dev Mints a specified amount of currency using external call.
     * It is wrapper over `mintCurrencyInternal` method to provide access to the owner of the account.
     * @param amount The amount of currency to mint.
     */
    function mintCurrency(uint256 amount) onlyExternal virtual public {
        mintCurrencyInternal(amount);
    }

    /**
     * @dev Sends a specified amount of arbitrary currency to a given address.
     * It is wrapper over `sendCurrencyInternal` method to provide access to the owner of the account.
     * @param amount The amount of currency to mint.
     */
    function sendCurrency(address to, uint256 currencyId, uint256 amount) onlyExternal virtual public {
        sendCurrencyInternal(to, currencyId, amount);
    }

    /**
     * @dev Mints a specified amount of currency and increases the total supply.
     * All minting should be carried out using this method.
     * @param amount The amount of currency to mint.
     */
    function mintCurrencyInternal(uint256 amount) internal {
        bool success = __Precompile__(Nil.MINT_CURRENCY).precompileMintCurrency(amount);
        require(success, "Mint failed");
        totalSupply += amount;
    }

    /**
     * @dev Sends a specified amount of arbitrary currency to a given address.
     * @param to The address to send the currency to.
     * @param currencyId ID of the currency to send.
     * @param amount The amount of currency to send.
     */
    function sendCurrencyInternal(address to, uint256 currencyId, uint256 amount) internal {
        Nil.Token[] memory tokens_ = new Nil.Token[](1);
        tokens_[0] = Nil.Token(currencyId, amount);
        Nil.asyncCall(to, address(0), address(0), 0, Nil.FORWARD_REMAINING, false, 0, tokens_, "");
    }

    /**
  * @dev Returns the balance of the currency for a given address.
     * @param account The address to check the balance for.
     * @return The balance of the currency for the given address.
     */
    function getCurrencyBalanceOf(address account) public view returns(uint256) {
        return Nil.currencyBalance(account, getCurrencyId());
    }
}