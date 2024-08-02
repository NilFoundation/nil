// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.9;

import "./Nil.sol";

contract Minter is NilBase {
    struct TokenInfo {
        uint256 id;
        string name;
        address owner;
        uint256 totalSupply;
    }
    bytes pubkey;
    mapping(uint256 => TokenInfo) public tokens;
    mapping(string => uint256) public namesMap;

    receive() external payable {}

    constructor(bytes memory _pubkey) {
        pubkey = _pubkey;
    }

    function create(uint256 amount, address owner, string memory name, address sendTo) onlyInternal payable public returns(bool) {
        if (owner == address(0)) {
            owner = msg.sender;
        }
        uint256 id = uint256(uint160(owner));
        require(id != 0, "Invalid token id");
        require(tokens[id].owner == address(0), "Token already exists");

        tokens[id] = TokenInfo(id, name, owner, 0);
        require(Nil.mintToken(id, amount), "Mint failed");
        tokens[id].totalSupply += amount;

        if (bytes(name).length != 0) {
            namesMap[name] = id;
        }

        if (sendTo != address(0)) {
            withdrawImpl(id, amount, sendTo);
        }

        return true;
    }

    function mint(uint256 id, uint256 amount, address sendTo) onlyInternal payable public {
        require(tokens[id].owner != address(0), "Token doesn't exist");
        require(msg.sender == tokens[id].owner, "Not from owner");
        Nil.mintToken(id, amount);
        tokens[id].totalSupply += amount;

        if (sendTo != address(0)) {
            withdrawImpl(id, amount, sendTo);
        }
    }

    function withdraw(uint256 id, uint256 amount, address to) onlyInternal payable public {
        require(tokens[id].owner != address(0), "Token doesn't exist");
        require(msg.sender == tokens[id].owner, "Not from owner");

        uint256 balance = Nil.tokensBalance(address(this), id);
        require(balance >= amount, "Insufficient balance");

        withdrawImpl(id, amount, to);
    }

    function withdrawImpl(uint256 id, uint256 amount, address to) onlyInternal internal {
        Nil.Token[] memory tokens_ = new Nil.Token[](1);
        tokens_[0] = Nil.Token(id, amount);

        if (Nil.getShardId(to) == Nil.getShardId(address(this))) {
            Nil.syncCall(to, gasleft(), 0, tokens_, "");
        } else {
            Nil.asyncCall(to, address(0), address(0), 0, Nil.FORWARD_REMAINING, false, 0, tokens_, "");
        }
    }

    // getName returns token name by its id.
    function getName(uint256 id) public view returns(string memory) {
        require(tokens[id].owner != address(0), "Token does not exist");
        return tokens[id].name;
    }

    // getIdByName returns token id by its name. If token with such name does not exist, returns 0.
    function getIdByName(string memory name) public view returns(uint256) {
        return namesMap[name];
    }

    function verifyExternal(uint256 hash, bytes memory signature) external view returns (bool) {
        return Nil.validateSignature(pubkey, hash, signature);
    }
}
