// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Nil.sol";
import "./NilTokenManager.sol";

/**
 * @title NilTokenBase
 * @dev Abstract contract that provides functionality for token processing.
 * Methods with "Internal" suffix are internal, which means that they can be called only from the derived contract
 * itself. But there are default wrapper methods that provide the account owner access to internal methods.
 * They are virtual, so the main contract can disable them by overriding them. Then only logic of the contract can use
 * internal methods.
 */
abstract contract NilTokenBase is NilBase, NilTokenHook {
    uint totalSupply;
    string tokenName;

    modifier onlyTokenManger() {
        require(msg.sender == Nil.getTokenManagerAddress(), "Only TokenManager can call this function");
        _;
    }

    function sendHook(address from, address to, address token, uint256 amount) external override virtual onlyTokenManger {
    }

    function receiveHook(address from, address to, address token, uint256 amount) external override virtual onlyTokenManger {
    }

    /**
     * @dev Returns the total supply of the token.
     * @return The total supply of the token.
     */
    function getTokenTotalSupply() public view returns(uint) {
        return NilTokenManager(Nil.getTokenManagerAddress()).totalSupply(address(this));
    }

    /**
     * @dev Returns the balance of the token owned by this contract.
     * @return The balance of the token owned by this contract.
     */
    function getOwnTokenBalance() public view returns(uint256) {
        return Nil.tokenBalance(Relayer(Nil.getRelayerAddress()).GetShardId(), address(this), getTokenId());
    }

    /**
     * @dev Returns the unique identifier of the token owned by this contract.
     * @return The unique identifier of the token owned by this contract.
     */
    function getTokenId() public view returns(TokenId) {
        return TokenId.wrap(address(this));
    }

    /**
     * @dev Returns the name of the token.
     * @return The name of the token.
     */
    function getTokenName() public view returns(string memory) {
        return NilTokenManager(Nil.getTokenManagerAddress()).getTokenName();
    }

    /**
     * @dev Set the name of the token.
     * @param name The name of the token.
     */
    function setTokenName(string memory name) onlyExternal virtual public {
        NilTokenManager(Nil.getTokenManagerAddress()).setTokenName(name);
    }

    /**
     * @dev Mints a specified amount of token using external call.
     * It is wrapper over `mintTokenInternal` method to provide access to the owner of the account.
     * @param amount The amount of token to mint.
     */
    function mintToken(uint256 amount) onlyExternal virtual public {
        mintTokenInternal(amount);
    }

    /**
     * @dev Burns a specified amount of token using external call.
     * It is wrapper over `burnTokenInternal` method to provide access to the owner of the account.
     * @param amount The amount of token to burn.
     */
    function burnToken(uint256 amount) onlyExternal virtual public {
        burnTokenInternal(amount);
    }

    /**
     * @dev Sends a specified amount of arbitrary token to a given address.
     * It is wrapper over `sendTokenInternal` method to provide access to the owner of the account.
     * @param amount The amount of token to mint.
     */
    function sendToken(uint256 shardIdDst, address to, TokenId tokenId, uint256 amount) onlyExternal virtual public {
        sendTokenInternal(shardIdDst, to, tokenId, amount);
    }

    /**
     * @dev Mints a specified amount of token and increases the total supply.
     * All minting should be carried out using this method.
     * @param amount The amount of token to mint.
     */
    function mintTokenInternal(uint256 amount) internal {
        NilTokenManager(Nil.getTokenManagerAddress()).mint(amount);
    }

    /**
     * @dev Burns a specified amount of token and decreases the total supply.
     * All burning should be carried out using this method.
     * @param amount The amount of token to mint.
     */
    function burnTokenInternal(uint256 amount) internal {
        NilTokenManager(Nil.getTokenManagerAddress()).burn(amount);
    }

    /**
     * @dev Sends a specified amount of arbitrary token to a given address.
     * @param to The address to send the token to.
     * @param tokenId ID of the token to send.
     * @param amount The amount of token to send.
     */
    function sendTokenInternal(uint256 shardIdDst, address to, TokenId tokenId, uint256 amount) internal {
        Nil.Token[] memory tokens_ = new Nil.Token[](1);
        tokens_[0] = Nil.Token(tokenId, amount);
        Nil.asyncCallWithTokens(shardIdDst, to, address(0), address(0), 0, Nil.FORWARD_REMAINING, 0, tokens_, "", 0, 0);
    }

    /**
     * @dev Returns the balance of the token for a given address.
     * @param account The address to check the balance for.
     * @return The balance of the token for the given address.
     */
    function getTokenBalanceOf(uint256 shardIdDst, address account) public view returns(uint256) {
        return Nil.tokenBalance(shardIdDst, account, getTokenId());
    }
}
