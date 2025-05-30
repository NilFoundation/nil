// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Nil.sol";
import "./IterableMapping.sol";

interface NilTokenHook {
    /**
     * @dev Hook that is called before tokens are sent.
     * @param from The address sending the tokens.
     * @param to The address receiving the tokens.
     * @param token The address of the token being transferred.
     * @param value The amount of tokens being transferred.
     */
    function sendHook(address from, address to, address token, uint256 value) external;

    /**
     * @dev Hook that is called after tokens are received.
     * @param from The address sending the tokens.
     * @param to The address receiving the tokens.
     * @param token The address of the token being transferred.
     * @param value The amount of tokens being transferred.
     */
    function receiveHook(address from, address to, address token, uint256 value) external;
}

/**
 * @title NilTokenManager
 * @dev Manages token balances, transfers, minting, and burning for the Nil ecosystem.
 */
contract NilTokenManager {

    /**
     * @dev Error thrown when a function is called by an account that is not the relayer.
     * @param account The address of the unauthorized account.
     */
    error CalledNotFromRelayer(address account);

    /**
     * @dev Emitted when tokens are minted.
     * @param sender The address that minted the tokens.
     * @param token The address of the token being minted.
     * @param value The amount of tokens minted.
     */
    event TokenMinted(address indexed sender, address indexed token, uint256 value);

    /**
     * @dev Emitted when tokens are burned.
     * @param sender The address that burned the tokens.
     * @param token The address of the token being burned.
     * @param value The amount of tokens burned.
     */
    event TokenBurned(address indexed sender, address indexed token, uint256 value);

    /**
     * @dev Struct to store token data, including balance.
     */
    struct TokenData {
        uint256 balance; // The balance of the token.
    }

    // Mapping of account addresses to their token balances.
    mapping(address => IterableMapping.Map) tokensMap;

    // Mapping of token addresses to their total supply.
    mapping(address => uint256) totalSupplyMap;

    // Mapping of token addresses to their names.
    mapping(address => string) public tokenNames;

    // Array to store tokens involved in the current transaction.
    Nil.Token[] private txTokens;

    /**
     * @dev Modifier to restrict access to functions that can only be called by the relayer.
     */
    modifier onlyRelayer() {
        if (msg.sender != Nil.getRelayerAddress()) {
            revert CalledNotFromRelayer(msg.sender);
        }
        _;
    }

    /**
     * @dev Calls the send hook for a token transfer.
     * @param from The address sending the tokens.
     * @param to The address receiving the tokens.
     * @param token The address of the token being transferred.
     * @param value The amount of tokens being transferred.
     */
    function callSendHook(address from, address to, address token, uint256 value) internal {
        address addr = Nil.getAddressForShard(msg.sender, Nil.getCurrentShardId());
        NilTokenHook(addr).sendHook(from, to, token, value);
    }

    /**
     * @dev Calls the receive hook for a token transfer.
     * @param from The address sending the tokens.
     * @param to The address receiving the tokens.
     * @param token The address of the token being transferred.
     * @param value The amount of tokens being transferred.
     */
    function callReceiveHook(address from, address to, address token, uint256 value) internal {
        address addr = Nil.getAddressForShard(msg.sender, Nil.getCurrentShardId());
        NilTokenHook(addr).receiveHook(from, to, token, value);
    }

    /**
     * @dev Deducts tokens from a sender's balance for a relay operation.
     * @param from The address sending the tokens.
     * @param token The address of the token being deducted.
     * @param value The amount of tokens to deduct.
     */
    function deductForRelay(address from, address /*to*/, address token, uint256 value) internal {
        uint256 balance = IterableMapping.get(tokensMap[from], token);
        require(balance >= value, "TokenManager: insufficient token balance for deductForRelay");
        IterableMapping.set(tokensMap[from], token, balance - value);
    }

    /**
     * @dev Deducts tokens from a sender's balance for a relay operation (overloaded version).
     * @param from The address sending the tokens.
     * @param to The address receiving the tokens.
     * @param tokens An array of tokens to deduct.
     */
    function deductForRelay(address from, address to, Nil.Token[] memory tokens) public onlyRelayer {
        for (uint256 i = 0; i < tokens.length; i++) {
            address token = TokenId.unwrap(tokens[i].id);
            uint256 value = tokens[i].amount;
            deductForRelay(from, to, token, value);
        }
    }

    /**
     * @dev Credits tokens to a recipient's balance for a relay operation.
     * @param to The address receiving the tokens.
     * @param token The address of the token being credited.
     * @param value The amount of tokens to credit.
     */
    function creditForRelay(address to, address token, uint256 value) internal {
        uint256 balance = IterableMapping.get(tokensMap[to], token);
        IterableMapping.set(tokensMap[to], token, balance + value);
    }

    /**
     * @dev Credits tokens to a recipient's balance for a relay operation (overloaded version).
     * @param to The address receiving the tokens.
     * @param tokens An array of tokens to credit.
     */
    function creditForRelay(address to, Nil.Token[] memory tokens) public onlyRelayer {
        for (uint256 i = 0; i < tokens.length; i++) {
            address token = TokenId.unwrap(tokens[i].id);
            uint256 value = tokens[i].amount;
            creditForRelay(to, token, value);
        }
        setTxTokens(tokens);
    }

    /**
     * @dev Transfers tokens from the sender to a specified address.
     * @param dst The address to transfer tokens to.
     * @param tokens An array of tokens to transfer.
     */
    function transfer(address dst, Nil.Token[] memory tokens) public {
        require(Nil.getShardId(address(msg.sender)) == Nil.getShardId(address(dst)), "Shard ID mismatch");
        for (uint i = 0; i < tokens.length; i++) {
            address token = TokenId.unwrap(tokens[i].id);

            uint256 oldValue = IterableMapping.get(tokensMap[msg.sender], token);
            require(oldValue >= tokens[i].amount, "Insufficient token balance for transfer");
            IterableMapping.set(tokensMap[msg.sender], token, oldValue - tokens[i].amount);

            uint256 oldValueDst = IterableMapping.get(tokensMap[dst], token);
            IterableMapping.set(tokensMap[dst], token, oldValueDst + tokens[i].amount);
        }
    }

    /**
     * @dev Transfers tokens and executes a call on the destination address.
     * @param dst The destination address.
     * @param gas The gas limit for the call.
     * @param value The Ether value to send with the call.
     * @param tokens An array of tokens to transfer.
     * @param callData The calldata for the call.
     * @return The return data from the call.
     */
    function transferCall(
        address dst,
        uint gas,
        uint value,
        Nil.Token[] memory tokens,
        bytes memory callData
    ) public returns(bytes memory) {
        require(Nil.getShardId(dst) == Nil.getCurrentShardId(), "transferCall: cross shard transfer is not allowed");
        transfer(dst, tokens);

        setTxTokens(tokens);
        (bool success, bytes memory returnData) = dst.call{gas: gas, value: value}(callData);
        _resetTxTokens();

        if (!success) {
            if (returnData.length > 68) {
                assembly {
                    returnData := add(returnData, 0x04)
                }
                string memory reason = abi.decode(returnData, (string));
                revert(reason);
            } else {
                revert("transferCall: call failed without revert reason");
            }
        }
        return returnData;
    }

    /**
     * @dev Burns a specified amount of tokens from the sender's balance.
     * @param value The amount of tokens to burn.
     */
    function burn(uint256 value) external {
        address token = msg.sender;
        uint256 balance = IterableMapping.get(tokensMap[msg.sender], token);
        require(balance >= value, "TokenManager: insufficient token balance for burn");

        IterableMapping.set(tokensMap[msg.sender], token, balance - value);
        totalSupplyMap[token] -= value;

        emit TokenBurned(msg.sender, token, value);
    }

    /**
     * @dev Mints a specified amount of tokens to the sender's balance.
     * @param value The amount of tokens to mint.
     */
    function mint(uint256 value) external {
        address token = msg.sender;
        uint256 balance = IterableMapping.get(tokensMap[msg.sender], token);

        IterableMapping.set(tokensMap[msg.sender], token, balance + value);
        totalSupplyMap[token] += value;

        emit TokenMinted(msg.sender, token, value);
    }

    /**
     * @dev Returns the total supply of a specified token.
     * @param token The address of the token.
     * @return The total supply of the token.
     */
    function totalSupply(address token) view external returns (uint256) {
        return totalSupplyMap[token];
    }

    /**
     * @dev Returns the tokens and their balances for a specified account.
     * @param account The address of the account.
     * @return An array of tokens and their balances.
     */
    function getTokens(address account) external view returns (Nil.Token[] memory) {
        uint256 length = IterableMapping.size(tokensMap[account]);
        Nil.Token[] memory tokens = new Nil.Token[](length);
        for (uint256 i = 0; i < length; i++) {
            address token = IterableMapping.getKeyAtIndex(tokensMap[account], i);
            uint256 value = IterableMapping.get(tokensMap[account], token);
            tokens[i] = Nil.Token({id: TokenId.wrap(token), amount: value});
        }
        return tokens;
    }

    /**
     * @dev Sets the tokens involved in the current transaction.
     * @param tokens An array of tokens to set.
     */
    function setTxTokens(Nil.Token[] memory tokens) internal {
        require(txTokens.length == 0, "TokenManager: txTokens already set, nested calls not allowed");
        for (uint256 i = 0; i < tokens.length; i++) {
            txTokens.push(tokens[i]);
        }
    }

    /**
     * @dev Resets the tokens involved in the current transaction.
     */
    function resetTxTokens() public onlyRelayer {
        _resetTxTokens();
    }

    function _resetTxTokens() internal {
        delete txTokens;
    }

    /**
     * @dev Returns the tokens involved in the current transaction.
     * @return An array of tokens involved in the transaction.
     */
    function getTxTokens() public view returns(Nil.Token[] memory) {
        Nil.Token[] memory tokens = new Nil.Token[](txTokens.length);
        for (uint256 i = 0; i < txTokens.length; i++) {
            tokens[i] = txTokens[i];
        }
        return tokens;
    }

    /**
     * @dev Returns the balance of a specified token for a given account.
     * @param account The address of the account.
     * @param token The address of the token.
     * @return The balance of the token for the account.
     */
    function getBalance(address account, address token) external view returns (uint256) {
        return IterableMapping.get(tokensMap[account], token);
    }

    /**
     * @dev Sets the name of the token for the sender.
     * @param name The name of the token.
     */
    function setTokenName(string memory name) public {
        tokenNames[msg.sender] = name;
    }

    /**
     * @dev Returns the name of the token for the sender.
     * @return The name of the token.
     */
    function getTokenName() public view returns (string memory) {
        return tokenNames[msg.sender];
    }
}