// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "../Nil.sol";
import "../Minter.sol";

contract TokensTest is NilBase {

    function createToken(uint256 amount, string memory name) onlyExternal public {
        bool success = Minter(Nil.MINTER_ADDRESS).create(amount, address(0), name, address(this));
        require(success, "Create token failed");
    }

    // Perform sync call to send tokens to the destination address. Without calling any function.
    function testSendTokensSync(address dst, uint256 amount, bool fail) onlyExternal public {
        Nil.Token[] memory tokens = new Nil.Token[](1);
        uint256 id = uint256(uint160(address(this)));
        tokens[0] = Nil.Token(id, amount);
        Nil.syncCall(dst, gasleft(), 0, tokens, "");
        require(!fail, "Test for failed transaction");
    }

    function testCallWithTokensSync(address dst, Nil.Token[] memory tokens) onlyExternal public {
        bytes memory callData = abi.encodeCall(this.testMessageTokens, tokens);
        (bool success,) = Nil.syncCall(dst, gasleft(), 0, tokens, callData);
        require(success, "Sync call failed");
    }

    function testCallWithTokensAsync(address dst, Nil.Token[] memory tokens) onlyExternal public {
        bytes memory callData = abi.encodeCall(this.testMessageTokens, tokens);
        uint256 gas = gasleft() * tx.gasprice;
        bool success = Nil.asyncCall(dst, address(0), address(0), gas, false, gas * 10, tokens, callData);
        require(success, "Async call failed");
    }

    function testMessageTokens(Nil.Token[] memory tokens) payable public {
        Nil.Token[] memory messageTokens = Nil.msgTokens();
        require(messageTokens.length == tokens.length, "Tokens length mismatch");
        for (uint i = 0; i < tokens.length; i++) {
            require(messageTokens[i].id == tokens[i].id, "Tokens id mismatch");
            require(messageTokens[i].amount == tokens[i].amount, "Tokens amount mismatch");
        }
    }

    function receiveTokens(bool fail) payable public {
        require(!fail, "Test for failed transaction");
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    receive() payable external {}
}
