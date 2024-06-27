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

        if (sendTo != address(0)) {
            transferImpl(id, amount, sendTo);
        }

        return true;
    }

    function mint(uint256 id, uint256 amount, address sendTo) onlyInternal payable public {
        require(tokens[id].owner != address(0), "Token doesn't exist");
        require(msg.sender == tokens[id].owner, "Not from owner");
        Nil.mintToken(id, amount);
        tokens[id].totalSupply += amount;

        if (sendTo != address(0)) {
            transferImpl(id, amount, sendTo);
        }
    }

    function transfer(uint256 id, uint256 amount, address to) onlyInternal payable public {
        require(tokens[id].owner != address(0), "Token doesn't exist");
        require(msg.sender == tokens[id].owner, "Not from owner");

        uint256 balance = Nil.tokensBalance(address(this), id);
        require(balance >= amount, "Insufficient balance");

        transferImpl(id, amount, to);
    }

    function transferImpl(uint256 id, uint256 amount, address to) onlyInternal internal {
        Nil.Token[] memory tokens_ = new Nil.Token[](1);
        tokens_[0] = Nil.Token(id, amount);

        uint256 gas = gasleft();
        Nil.asyncCall(to, address(0), address(0), gas, false, gas * 10, tokens_, "");
    }

    function getName(uint256 id) public view returns(string memory) {
        require(tokens[id].owner != address(0), "Token does not exist");
        return tokens[id].name;
    }

    function verifyExternal(uint256 hash, bytes memory signature) external view returns (bool) {
        return Nil.validateSignature(pubkey, hash, signature);
    }
}
