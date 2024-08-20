// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "../NilCurrencyBase.sol";

contract TokensTest is NilCurrencyBase {

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
        Nil.asyncCall(dst, address(0), address(0), gas, Nil.FORWARD_NONE, false, 0, tokens, callData);
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

    function checkTokenBalance(address addr, uint256 id, uint256 balance) public {
        require(Nil.currencyBalance(addr, id) == balance, "Balance mismatch");
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    receive() payable external {}
}

contract TokensTestNoExternalAccess is NilCurrencyBase {
    function setCurrencyName(string memory) onlyExternal view override public {
        revert("Not allowed");
    }

    function mintCurrency(uint256) onlyExternal view override public {
        revert("Not allowed");
    }

    function sendCurrency(address, uint256, uint256) onlyExternal view override public {
        revert("Not allowed");
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }
}