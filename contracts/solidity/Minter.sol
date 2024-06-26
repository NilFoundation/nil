// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.9;

import "./Nil.sol";

contract Minter is NilBase {
    bytes pubkey;
    mapping(uint256 => address) public owners;

    receive() external payable {}

    constructor(bytes memory _pubkey) {
        pubkey = _pubkey;
    }

    function create(uint256 amount, address owner) onlyInternal payable public returns(bool) {
        if (owner == address(0)) {
            owner = msg.sender;
        }
        uint256 id = uint256(uint160(owner));
        require(id != 0, "Invalid token id");
        require(owners[id] == address(0), "Token already exists");

        owners[id] = owner;
        require(Nil.mintToken(id, amount), "Mint failed");

        return true;
    }

    function mint(uint256 id, uint256 amount) onlyInternal payable public {
        require(owners[id] != address(0), "Token doesn't exist");
        require(msg.sender == owners[id], "Not from owner");
        Nil.mintToken(id, amount);
    }

    function transfer(uint256 id, uint256 amount, address to) onlyInternal payable public {
        require(owners[id] != address(0), "Token doesn't exist");
        require(msg.sender == owners[id], "Not from owner");

        uint256 balance = Nil.getTokenBalance(id);
        require(balance >= amount, "Insufficient balance");

        bytes memory emptyData;
        Nil.Token[] memory tokens = new Nil.Token[](1);
        tokens[0] = Nil.Token(id, amount);

        Nil.asyncCall(to, address(0), address(0), gasleft(), false, msg.value, tokens, emptyData);
    }

    function verifyExternal(uint256 hash, bytes memory signature) external view returns (bool) {
        return Nil.validateSignature(pubkey, hash, signature);
    }
}
