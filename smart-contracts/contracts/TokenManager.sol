// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "./Nil.sol";
import "./IterableMapping.sol";

contract TokenManager {

    event TokenMinted(address indexed sender, address indexed token, uint256 amount);
    event TokenBurned(address indexed sender, address indexed token, uint256 amount);

    mapping(address => IterableMapping.Map) tokensMap;
    mapping(address => uint256) totalSupplyMap;

    function deduct(address from, address token, uint256 amount) external {
        require(Nil.getRelayerAddress() == msg.sender, "Only relayer can deduct");
        uint256 oldValue = IterableMapping.get(tokensMap[from], token);
        require(oldValue >= amount, "Insufficient token balance");
        IterableMapping.set(tokensMap[from], token, oldValue - amount);
    }

    function credit(address to, address token, uint256 amount) external {
        require(Nil.getRelayerAddress() == msg.sender, "Only relayer can credit");
        uint256 oldValue = IterableMapping.get(tokensMap[to], token);
        IterableMapping.set(tokensMap[to], token, oldValue + amount);
    }

    function burn(address token, uint256 amount) external {
        require(msg.sender == token, "Only owner can burn");
        uint256 oldValue = IterableMapping.get(tokensMap[msg.sender], token);
        require(oldValue >= amount, "Insufficient token balance");

        IterableMapping.set(tokensMap[msg.sender], token, oldValue - amount);
        totalSupplyMap[token] -= amount;

        emit TokenBurned(msg.sender, token, amount);
    }

    function mint(address token, uint256 amount) external {
        require(msg.sender == token, "Only owner can mint");
        uint256 oldValue = IterableMapping.get(tokensMap[msg.sender], token);

        IterableMapping.set(tokensMap[msg.sender], token, oldValue + amount);
        totalSupplyMap[token] += amount;

        emit TokenMinted(msg.sender, token, amount);
    }

    function transfer(
        address dst,
        Nil.Token[] memory tokens
    ) public {
        require(Nil.getShardId(address(msg.sender)) == Nil.getShardId(address(dst)), "Shard ID mismatch");
        for (uint i = 0; i < tokens.length; i++) {
            address token = TokenId.unwrap(tokens[i].id);

            uint256 oldValue = IterableMapping.get(tokensMap[msg.sender], token);
            require(oldValue >= tokens[i].amount, "Insufficient token balance");
            IterableMapping.set(tokensMap[msg.sender], token, oldValue - tokens[i].amount);

            uint256 oldValueDst = IterableMapping.get(tokensMap[dst], token);
            IterableMapping.set(tokensMap[dst], token, oldValueDst + tokens[i].amount);
        }
    }

    function totalSupply(address token) view external returns (uint256) {
        return totalSupplyMap[token];
    }

    function getTokens(address account) external view returns (Nil.Token[] memory) {
        uint256 length = IterableMapping.size(tokensMap[account]);
        Nil.Token[] memory tokens = new Nil.Token[](length);
        for (uint256 i = 0; i < length; i++) {
            address token = IterableMapping.getKeyAtIndex(tokensMap[account], i);
            uint256 amount = IterableMapping.get(tokensMap[account], token);
            tokens[i] = Nil.Token({id: TokenId.wrap(token), amount: amount});
        }
        return tokens;
    }

    function getToken(address account, address token) external view returns (uint256) {
        return IterableMapping.get(tokensMap[account], token);
    }
}