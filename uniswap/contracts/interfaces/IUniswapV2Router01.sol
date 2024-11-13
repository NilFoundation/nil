// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;


interface IUniswapV2Router01 {

    function addLiquidity(
        address pair,
        address to
    ) external;
    function addLiquiditySync(
        address pair,
        address to,
        uint amountADesired,
        uint amountBDesired,
        uint amountAMin,
        uint amountBMin
    ) external returns (uint amountA, uint amountB);
    function removeLiquidity(
        address pair,
        address to
    ) external;
    function removeLiquiditySync(
        address pair,
        address to,
        uint amountAMin,
        uint amountBMin
    ) external returns (uint amountA, uint amountB);
    function swap(
        address to,
        address pair,
        uint amount0Out,
        uint amount1Out
    ) external;

    function swapExactTokenForTokenSync(
        address pair,
        uint amountOutMin,
        address to
    ) external returns (uint amount);

    function quote(uint amountA, uint reserveA, uint reserveB) external pure returns (uint amountB);
    function getAmountOut(uint amountIn, uint reserveIn, uint reserveOut) external pure returns (uint amountOut);
    function getAmountIn(uint amountOut, uint reserveIn, uint reserveOut) external pure returns (uint amountIn);
}
