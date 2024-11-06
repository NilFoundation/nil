// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import './interfaces/IUniswapV2Router01.sol';
import './interfaces/IUniswapV2Factory.sol';
import './libraries/UniswapV2Library.sol';
import "@nilfoundation/smart-contracts/contracts/NilCurrencyBase.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "./interfaces/IUniswapV2Pair.sol";

contract UniswapV2Router01 is IUniswapV2Router01, NilCurrencyBase {

    modifier sameShard(address _addr) {
        require(Nil.getShardId(_addr) == Nil.getShardId(address(this)), "Sync calls require same shard for all contracts");
        _;
    }

    function addLiquidity(
        address pair,
        address to
    ) public override {
        Nil.Token[] memory tokens = Nil.msgTokens();
        if (tokens.length != 2) {
            revert("Send only 2 tokens to add liquidity");
        }
        smartCall(pair, tokens, abi.encodeWithSignature("mint(address)", to));
    }

    function addLiquiditySync(
        address pair,
        address to,
        uint amountADesired,
        uint amountBDesired,
        uint amountAMin,
        uint amountBMin
    ) public override sameShard(pair) returns (uint amountA, uint amountB) {
        Nil.Token[] memory tokens = Nil.msgTokens();
        if (tokens.length != 2) {
            revert("Send only 2 tokens to add liquidity");
        }
        (amountA, amountB) = _addLiquiditySync(pair, tokens[0].id, tokens[1].id, amountADesired, amountBDesired, amountAMin, amountBMin);

        if (amountA < tokens[0].amount) {
            Nil.Token[] memory tokenAReturns = new Nil.Token[](1);
            tokenAReturns[0].id = tokens[0].id;
            tokenAReturns[0].amount = tokens[0].amount - amountA;
            smartCall(to, tokenAReturns, "");
        }
        if (amountB < tokens[1].amount) {
            Nil.Token[] memory tokenBReturns = new Nil.Token[](1);
            tokenBReturns[0].id = tokens[1].id;
            tokenBReturns[0].amount = tokens[1].amount - amountB;
            smartCall(to, tokenBReturns, "");
        }

        Nil.Token[] memory tokensToSend = new Nil.Token[](2);
        tokensToSend[0].id = tokens[0].id;
        tokensToSend[0].amount = amountA;
        tokensToSend[1].id = tokens[1].id;
        tokensToSend[1].amount = amountA;
        smartCall(pair, tokensToSend, abi.encodeWithSignature("mint(address)", to));
    }

    function _addLiquiditySync(
        address pair,
        CurrencyId tokenA,
        CurrencyId tokenB,
        uint amountADesired,
        uint amountBDesired,
        uint amountAMin,
        uint amountBMin
    ) private returns (uint amountA, uint amountB) {
        (uint reserveA, uint reserveB) = UniswapV2Library.getReserves(pair, tokenA, tokenB);
        if (reserveA == 0 && reserveB == 0) {
            (amountA, amountB) = (amountADesired, amountBDesired);
        } else {
            uint amountBOptimal = UniswapV2Library.quote(amountADesired, reserveA, reserveB);
            if (amountBOptimal <= amountBDesired) {
                require(amountBOptimal >= amountBMin, 'UniswapV2Router: INSUFFICIENT_B_AMOUNT');
                (amountA, amountB) = (amountADesired, amountBOptimal);
            } else {
                uint amountAOptimal = UniswapV2Library.quote(amountBDesired, reserveB, reserveA);
                assert(amountAOptimal <= amountADesired);
                require(amountAOptimal >= amountAMin, 'UniswapV2Router: INSUFFICIENT_A_AMOUNT');
                (amountA, amountB) = (amountAOptimal, amountBDesired);
            }
        }
    }

    // **** REMOVE LIQUIDITY ****
    function removeLiquidity(
        address pair,
        address to
    ) public override {
        Nil.Token[] memory tokens = Nil.msgTokens();
        if (tokens.length != 1) {
            revert("UniswapV2Router: should contains only pair token");
        }
        smartCall(pair, tokens, abi.encodeWithSignature("burn(address)", to));
    }

    function removeLiquiditySync(
        address pair,
        address to,
        uint amountAMin,
        uint amountBMin
    ) public override sameShard(pair) returns (uint amountA, uint amountB) {
        Nil.Token[] memory tokens = Nil.msgTokens();
        if (tokens.length != 1) {
            revert("UniswapV2Router: should contains only pair token");
        }
        (bool success, bytes memory result) = smartCall(pair, tokens, abi.encodeWithSignature("burn(address)", to));
        if (success) {
            (amountA, amountB) = abi.decode(result, (uint256, uint256));
        } else {
            revert("Burn call is not successful");
        }
    }

    function swap(address to, address pair, uint amount0Out, uint amount1Out) public override {
        Nil.Token[] memory tokens = Nil.msgTokens();
        if (tokens.length != 1) {
            revert("UniswapV2Router: should contains only pair token");
        }
        smartCall(pair, tokens, abi.encodeWithSignature("swap(uint256,uint256,address)", amount0Out, amount1Out, to));
    }

    function swapExactTokenForTokenSync(
        address pair,
        uint amountOutMin,
        address to
    ) external override sameShard(pair) returns (uint amount) {
        Nil.Token[] memory tokens = Nil.msgTokens();
        if (tokens.length != 1) {
            revert("UniswapV2Router: should contains only pair token");
        }
        CurrencyId token0Id = IUniswapV2Pair(pair).token0Id();
        CurrencyId token1Id = IUniswapV2Pair(pair).token1Id();
        CurrencyId tokenBId = tokens[0].id != token0Id ? token0Id : token1Id;
        (uint reserveA, uint reserveB) = UniswapV2Library.getReserves(pair, tokens[0].id, tokenBId);
        amount = UniswapV2Library.getAmountOut(tokens[0].amount, reserveA, reserveB);
        require(amount >= amountOutMin, 'UniswapV2Router: INSUFFICIENT_OUTPUT_AMOUNT');
        uint amount0Out = tokens[0].id == token0Id ? 0 : amount;
        uint amount1Out = tokens[0].id != token0Id ? 0 : amount;
        (bool success, bytes memory result) = smartCall(pair, tokens, abi.encodeWithSignature("swap(uint256,uint256,address)", amount0Out, amount1Out, to));
        if (!success) {
            revert("UniswapV2Router: should get success swap result");
        }
    }

    function quote(uint amountA, uint reserveA, uint reserveB) public pure override returns (uint amountB) {
        return UniswapV2Library.quote(amountA, reserveA, reserveB);
    }

    function getAmountOut(uint amountIn, uint reserveIn, uint reserveOut) public pure override returns (uint amountOut) {
        return UniswapV2Library.getAmountOut(amountIn, reserveIn, reserveOut);
    }

    function getAmountIn(uint amountOut, uint reserveIn, uint reserveOut) public pure override returns (uint amountIn) {
        return UniswapV2Library.getAmountOut(amountOut, reserveIn, reserveOut);
    }

    receive() external payable {
    }

    function smartCall(address dst, Nil.Token[] memory tokens, bytes memory callData) private returns (bool, bytes memory) {
        if (Nil.getShardId(dst) == Nil.getShardId(address(this))) {
            (bool success, bytes memory result) = Nil.syncCall(dst, gasleft(), 0, tokens, callData);
            return (success, result);
        } else {
            Nil.asyncCallWithTokens(dst, address(0), address(0), 0, Nil.FORWARD_REMAINING, false, 0, tokens, callData);
            return (true, "");
        }
    }
}