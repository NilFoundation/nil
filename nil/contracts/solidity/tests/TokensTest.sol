// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "../lib/NilTokenBase.sol";
import "../lib/console.sol";

contract TokensTest is NilTokenBase {
    // Perform sync call to send tokens to the destination address. Without calling any function.
    function testSendTokensSync(
        address dst,
        uint256 amount,
        bool fail
    ) public onlyExternal {
        Nil.Token[] memory tokens = new Nil.Token[](1);
        TokenId id = TokenId.wrap(address(this));
        tokens[0] = Nil.Token(id, amount);
        Nil.syncCall(dst, gasleft(), 0, tokens, "");
        require(!fail, "Test for failed transaction");
    }

    function testCallWithTokensSync(
        address dst,
        Nil.Token[] memory tokens
    ) public onlyExternal {
        bytes memory callData = abi.encodeCall(
            this.testTransactionTokens,
            tokens
        );
        (bool success, ) = Nil.syncCall(dst, gasleft(), 0, tokens, callData);
        require(success, "Sync call failed");
    }

    function testCallWithTokensAsync(
        address dst,
        Nil.Token[] memory tokens
    ) public onlyExternal {
        bytes memory callData = abi.encodeCall(
            this.testTransactionTokens,
            tokens
        );
        uint256 gas = gasleft() * tx.gasprice;
        Nil.asyncCallWithTokens(
            dst,
            address(0),
            address(0),
            gas,
            Nil.FORWARD_REMAINING,
            0,
            tokens,
            callData
        );
    }

    function testAsyncDeployWithTokens(
        uint shardId,
        uint feeCredit,
        uint value,
        bytes memory code,
        uint256 salt,
        Nil.Token[] memory tokens
    ) public onlyExternal returns (address) {
        address contractAddress = Nil.createAddress(shardId, code, salt);
        // 0xfd == Nil.ASYNC_CALL
        __Precompile__(address(0xfd)).precompileAsyncCall{value: value}(
            true,
            Nil.FORWARD_REMAINING,
            contractAddress,
            address(0),
            address(this),
            feeCredit,
            tokens,
            bytes.concat(code, bytes32(salt))
        );
        return contractAddress;
    }

    function testTransactionTokens(Nil.Token[] memory tokens) public payable {
        // TODO: [PoC] Tokens fix it
    }

    function receiveTokens(bool fail) public payable {
        require(!fail, "Test for failed transaction");
    }

    function checkTokenBalance(
        address addr,
        TokenId id,
        uint256 balance
    ) public view {
        require(Nil.tokenBalance(addr, id) == balance, "Balance mismatch");
    }

    function testConsole() public pure {
        console.log("test console.log: int=%_, str=%_, addr=%_",
            1234567890,
            "Simple string",
            address(0xabcdef)
        );
    }

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
    }

    event tokenBalance(uint256 balance);
    event tokenTxnBalance(uint256 balance);

    function checkIncomingToken(TokenId id) public payable {
        Nil.Token[] memory tokens = Nil.txnTokens();
        require(tokens.length == 1, "Expected one token in transaction");
        emit tokenTxnBalance(tokens[0].amount);
        emit tokenBalance(Nil.tokenBalance(address(this), id));
    }

    receive() external payable {}
}
