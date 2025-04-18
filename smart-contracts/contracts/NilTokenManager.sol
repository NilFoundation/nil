// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "./Nil.sol";
import "./IterableMapping.sol";
import "./console.sol";

interface NilTokenHook {
    function sendHook(address from, address to, address token, uint256 value) external;
    function receiveHook(address from, address to, address token, uint256 value) external;
}

contract NilTokenManager {

    error CalledNotFromRelayer(address account);

    event TokenMinted(address indexed sender, address indexed token, uint256 value);
    event TokenBurned(address indexed sender, address indexed token, uint256 value);

    struct TokenData {
        uint256 balance;
    }

    mapping(address => IterableMapping.Map) tokensMap;
    mapping(address => uint256) totalSupplyMap;
    mapping(address => string) public tokenNames;
    Nil.Token[] private txTokens;
    uint txTokensBlockNumber;

    modifier onlyRelayer() {
        if (msg.sender != Nil.getRelayerAddress()) {
            revert CalledNotFromRelayer(msg.sender);
        }
        _;
    }

    function callSendHook(address from, address to, address token, uint256 value) internal {
        address addr = Nil.getAddressForShard(msg.sender, Nil.getCurrentShardId());
        NilTokenHook(addr).sendHook(from, to, token, value);
    }

    function callReceiveHook(address from, address to, address token, uint256 value) internal {
        address addr = Nil.getAddressForShard(msg.sender, Nil.getCurrentShardId());
        NilTokenHook(addr).receiveHook(from, to, token, value);
    }

    function deductForRelay(address from, address /*to*/, address token, uint256 value) internal {
        uint256 balance = IterableMapping.get(tokensMap[from], token);
        require(balance >= value, "TokenManager: insufficient token balance");
        IterableMapping.set(tokensMap[from], token, balance - value);
    }

    function deductForRelay(address from, address to, Nil.Token[] memory tokens) public onlyRelayer {
        for (uint256 i = 0; i < tokens.length; i++) {
            console.log("deductForRelay: token=%_, value=%_", TokenId.unwrap(tokens[i].id), tokens[i].amount);
            address token = TokenId.unwrap(tokens[i].id);
            uint256 value = tokens[i].amount;
            deductForRelay(from, to, token, value);
        }
    }

    function creditForRelay(address to, address token, uint256 value) internal {
        uint256 balance = IterableMapping.get(tokensMap[to], token);
        IterableMapping.set(tokensMap[to], token, balance + value);
    }

    function creditForRelay(address to, Nil.Token[] memory tokens) public onlyRelayer {
        for (uint256 i = 0; i < tokens.length; i++) {
            console.log("creditForRelay: token=%_, value=%_", TokenId.unwrap(tokens[i].id), tokens[i].amount);
            address token = TokenId.unwrap(tokens[i].id);
            uint256 value = tokens[i].amount;
            creditForRelay(to, token, value);
        }
        setTxTokens(tokens);
    }

    function transfer(
        address dst,
        Nil.Token[] memory tokens
    ) internal {
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
        resetTxTokens();
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

    function burn(uint256 value) external {
        address token = msg.sender;
        uint256 balance = IterableMapping.get(tokensMap[msg.sender], token);
        require(balance >= value, "TokenManager: insufficient token balance");

        IterableMapping.set(tokensMap[msg.sender], token, balance - value);
        totalSupplyMap[token] -= value;

        emit TokenBurned(msg.sender, token, value);
    }

    function mint(uint256 value) external {
        address token = msg.sender;
        uint256 balance = IterableMapping.get(tokensMap[msg.sender], token);

        IterableMapping.set(tokensMap[msg.sender], token, balance + value);
        totalSupplyMap[token] += value;

        emit TokenMinted(msg.sender, token, value);
    }

    function totalSupply(address token) view external returns (uint256) {
        return totalSupplyMap[token];
    }

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

    function setTxTokens(Nil.Token[] memory tokens) internal {
        require(txTokens.length == 0, "TokenManager: txTokens already set, nested calls not allowed");
        for (uint256 i = 0; i < tokens.length; i++) {
            txTokens.push(tokens[i]);
        }
        txTokensBlockNumber = block.number;
    }

    function resetTxTokens() public {
        delete txTokens;
    }

    function getTxTokens() public view returns(Nil.Token[] memory) {
        if (txTokensBlockNumber == block.number) {
            Nil.Token[] memory tokens = new Nil.Token[](txTokens.length);
            for (uint256 i = 0; i < txTokens.length; i++) {
                tokens[i] = txTokens[i];
            }
            return tokens;
        }
        return new Nil.Token[](0);
    }

    function getBalance(address account, address token) external view returns (uint256) {
        return IterableMapping.get(tokensMap[account], token);
    }

    function setTokenName(string memory name) public {
        tokenNames[msg.sender] = name;
    }

    function getTokenName() public view returns (string memory) {
        return tokenNames[msg.sender];
    }
}