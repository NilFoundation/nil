pragma solidity >=0.5.0;

import {Currency} from "../Currency.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";

interface IUniswapV2Pair {
    event Mint(address indexed sender, uint amount0, uint amount1);
    event Burn(
        address indexed sender,
        uint amount0,
        uint amount1,
        address indexed to
    );
    event Swap(
        address indexed sender,
        uint amount0In,
        uint amount1In,
        uint amount0Out,
        uint amount1Out,
        address indexed to
    );
    event Sync(uint256 reserve0, uint256 reserve1);
    function MINIMUM_LIQUIDITY() external pure returns (uint);
    function factory() external view returns (address);
    function token0() external view returns (address);
    function token1() external view returns (address);
    function token0Id() external view returns (CurrencyId);
    function token1Id() external view returns (CurrencyId);
    function getReserves()
        external
        view
        returns (uint256 reserve0, uint256 reserve1);
    function price0CumulativeLast() external view returns (uint);
    function price1CumulativeLast() external view returns (uint);
    function kLast() external view returns (uint);
    function mint(address to) external returns (uint liquidity);
    function burn(
        address to
    ) external returns (uint amount0, uint amount1);
    function swap(
        uint amount0Out,
        uint amount1Out,
        address to
    ) external;
    function skim(address to) external;
    function sync() external;
    function initialize(address, address, CurrencyId, CurrencyId) external;
}
