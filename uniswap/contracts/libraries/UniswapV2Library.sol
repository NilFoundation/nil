// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./SafeMath.sol";
import "../interfaces/IUniswapV2Pair.sol";

library UniswapV2Library {
    using SafeMath for uint;

    // returns sorted token addresses, used to handle return values from pairs sorted in this order
    function sortTokens(CurrencyId tokenAId, CurrencyId tokenBId) internal pure returns (CurrencyId token0, CurrencyId token1) {
        require(tokenAId != tokenBId, 'UniswapV2Library: IDENTICAL_ADDRESSES');
        (token0, token1) = CurrencyId.unwrap(tokenAId) < CurrencyId.unwrap(tokenBId) ? (tokenAId, tokenBId) : (tokenBId, tokenAId);
        require(token0 != CurrencyId.wrap(address(0)), 'UniswapV2Library: ZERO_ADDRESS');
    }

    // fetches and sorts the reserves for a pair
    function getReserves(address pair, CurrencyId tokenAId, CurrencyId tokenBId) internal view returns (uint reserveA, uint reserveB) {
        (CurrencyId token0,) = sortTokens(tokenAId, tokenBId);
        (uint reserve0, uint reserve1) = IUniswapV2Pair(pair).getReserves();
        (reserveA, reserveB) = tokenAId == token0 ? (reserve0, reserve1) : (reserve1, reserve0);
    }

    // given some amount of an asset and pair reserves, returns an equivalent amount of the other asset
    function quote(uint amountA, uint reserveA, uint reserveB) internal pure returns (uint amountB) {
        require(amountA > 0, 'UniswapV2Library: INSUFFICIENT_AMOUNT');
        require(reserveA > 0 && reserveB > 0, 'UniswapV2Library: INSUFFICIENT_LIQUIDITY');
        amountB = amountA.mul(reserveB) / reserveA;
    }

    // given an input amount of an asset and pair reserves, returns the maximum output amount of the other asset
    function getAmountOut(uint amountIn, uint reserveIn, uint reserveOut) internal pure returns (uint amountOut) {
        require(amountIn > 0, 'UniswapV2Library: INSUFFICIENT_INPUT_AMOUNT');
        require(reserveIn > 0 && reserveOut > 0, 'UniswapV2Library: INSUFFICIENT_LIQUIDITY');
        uint amountInWithFee = amountIn.mul(997);
        uint numerator = amountInWithFee.mul(reserveOut);
        uint denominator = reserveIn.mul(1000).add(amountInWithFee);
        amountOut = numerator / denominator;
    }

    // given an output amount of an asset and pair reserves, returns a required input amount of the other asset
    function getAmountIn(uint amountOut, uint reserveIn, uint reserveOut) internal pure returns (uint amountIn) {
        require(amountOut > 0, 'UniswapV2Library: INSUFFICIENT_OUTPUT_AMOUNT');
        require(reserveIn > 0 && reserveOut > 0, 'UniswapV2Library: INSUFFICIENT_LIQUIDITY');
        uint numerator = reserveIn.mul(amountOut).mul(1000);
        uint denominator = reserveOut.sub(amountOut).mul(997);
        amountIn = (numerator / denominator).add(1);
    }
}