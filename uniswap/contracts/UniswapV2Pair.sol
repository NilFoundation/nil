// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./interfaces/IUniswapV2Pair.sol";
import "./libraries/Math.sol";
import "./interfaces/IUniswapV2Factory.sol";
import "@nilfoundation/smart-contracts/contracts/NilCurrencyBase.sol";
import "./libraries/SafeMath.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import {Currency} from "./Currency.sol";

contract UniswapV2Pair is NilCurrencyBase, IUniswapV2Pair {
    using SafeMath for uint;
    uint public constant MINIMUM_LIQUIDITY = 10 ** 3;
    bytes4 private constant SELECTOR =
        bytes4(keccak256(bytes("transfer(address,uint256)")));

    address public factory;
    address public token0;
    address public token1;
    CurrencyId public tokenId0;
    CurrencyId public tokenId1;

    uint256 private reserve0; // uses single storage slot, accessible via getReserves
    uint256 private reserve1; // uses single storage slot, accessible via getReserves
    uint32 private blockTimestampLast; // uses single storage slot, accessible via getReserves

    uint public price0CumulativeLast;
    uint public price1CumulativeLast;
    uint public kLast; // reserve0 * reserve1, as of immediately after the most recent liquidity event

    uint private unlocked = 1;
    modifier lock() {
        require(unlocked == 1, "UniswapV2: LOCKED");
        unlocked = 0;
        _;
        unlocked = 1;
    }

    function getReserves()
        public
        view
        returns (uint256 _reserve0, uint256 _reserve1)
    {
        _reserve0 = reserve0;
        _reserve1 = reserve1;
    }

    function _safeTransfer(
        CurrencyId _tokenId,
        address _to,
        uint _value
    ) private {
        sendCurrencyInternal(_to, _tokenId, _value);
    }

    constructor() payable {
        factory = msg.sender;
    }

    // called once by the factory at time of deployment
    function initialize(
        address _token0,
        address _token1,
        CurrencyId _tokenId0,
        CurrencyId _tokenId1
    ) public {
        token0 = _token0;
        token1 = _token1;
        tokenId0 = _tokenId0;
        tokenId1 = _tokenId1;
    }

    // update reserves and, on the first call per block, price accumulators
    function _update(
        uint balance0,
        uint balance1,
        uint256 /*_reserve0*/,
        uint256 /*_reserve1*/
    ) internal {
        uint32 blockTimestamp = uint32(block.timestamp % 2 ** 32);

        reserve0 = balance0;
        reserve1 = balance1;
        blockTimestampLast = blockTimestamp;
    }

    // if fee is on, mint liquidity equivalent to 1/6th of the growth in sqrt(k)
    function _mintFee(
        uint256 _reserve0,
        uint256 _reserve1
    ) private returns (bool feeOn) {
        address feeTo = address(0);
        feeOn = false;
        uint _kLast = kLast; // gas savings
        if (feeOn) {
            if (_kLast != 0) {
                uint rootK = Math.sqrt(uint(_reserve0).mul(_reserve1));
                uint rootKLast = Math.sqrt(_kLast);
                if (rootK > rootKLast) {
                    uint numerator = totalSupply.mul(rootK.sub(rootKLast));
                    uint denominator = rootK.mul(5).add(rootKLast);
                    uint liquidity = numerator / denominator;
                    if (liquidity > 0) mintCurrencyInternal(liquidity);
                    sendCurrencyInternal(feeTo, getCurrencyId(), liquidity);
                }
            }
        } else if (_kLast != 0) {
            kLast = 0;
        }
    }

    // this low-level function should be called from a contract which performs important safety checks
    function mint(address to) public lock returns (uint liquidity) {
        (uint256 _reserve0, uint256 _reserve1) = getReserves(); // gas savings
        uint balance0 = Nil.currencyBalance(address(this), tokenId0);
        uint balance1 = Nil.currencyBalance(address(this), tokenId1);

        uint amount0 = balance0.sub(_reserve0);
        uint amount1 = balance1.sub(_reserve1);
        uint _totalSupply = totalSupply; // gas savings, must be defined here since totalSupply can update in _mintFee

        if (_totalSupply == 0) {
            liquidity = Math.sqrt(amount0.mul(amount1)).sub(MINIMUM_LIQUIDITY);
            totalSupply = totalSupply + liquidity; // permanently lock the first MINIMUM_LIQUIDITY
        } else {
            liquidity = Math.min(
                amount0.mul(_totalSupply) / _reserve0,
                amount1.mul(_totalSupply) / _reserve1
            );
        }
        require(liquidity > 0, "UniswapV2: INSUFFICIENT_LIQUIDITY_MINTED");

        mintCurrencyInternal(liquidity);
        sendCurrencyInternal(to, getCurrencyId(), liquidity);
        _update(balance0, balance1, _reserve0, _reserve1);
        // if (feeOn) kLast = uint(reserve0).mul(reserve1); // reserve0 and reserve1 are p-to-date
        emit Mint(msg.sender, amount0, amount1);
    }

    // this low-level function should be called from a contract which performs important safety checks
    function burn(address to) public lock returns (uint amount0, uint amount1) {
        (uint256 _reserve0, uint256 _reserve1) = getReserves(); // gas savings
        CurrencyId _tokenId0 = tokenId0; // gas savings
        CurrencyId _tokenId1 = tokenId1; // gas savings

        uint balance0 = Nil.currencyBalance(address(this), _tokenId0);
        uint balance1 = Nil.currencyBalance(address(this), _tokenId1);
        uint liquidity = Nil.currencyBalance(address(this), getCurrencyId());

        bool feeOn = _mintFee(_reserve0, _reserve1);
        amount0 = liquidity.mul(balance0) / totalSupply; // using balances ensures pro-rata distribution
        amount1 = liquidity.mul(balance1) / totalSupply; // using balances ensures pro-rata distribution
        require(
            amount0 > 0 && amount1 > 0,
            "UniswapV2: INSUFFICIENT_LIQUIDITY_BURNED"
        );
        burnCurrencyInternal(liquidity);
        _safeTransfer(_tokenId0, to, amount0);
        _safeTransfer(_tokenId1, to, amount1);

        balance0 = Nil.currencyBalance(address(this), _tokenId0);
        balance1 = Nil.currencyBalance(address(this), _tokenId1);
        _update(balance0, balance1, _reserve0, _reserve1);
        if (feeOn) kLast = uint(reserve0).mul(reserve1); // reserve0 and reserve1 are up-to-date
        emit Burn(msg.sender, amount0, amount1, to);
    }

    // this low-level function should be called from a contract which performs important safety checks
    function swap(uint amount0Out, uint amount1Out, address to) public lock {
        require(
            amount0Out > 0 || amount1Out > 0,
            "UniswapV2: INSUFFICIENT_OUTPUT_AMOUNT"
        );
        (uint256 _reserve0, uint256 _reserve1) = getReserves(); // gas savings
        require(
            amount0Out < _reserve0 && amount1Out < _reserve1,
            "UniswapV2: INSUFFICIENT_LIQUIDITY"
        );
        uint balance0;
        uint balance1;
        balance0 = Nil.currencyBalance(address(this), tokenId0);
        balance1 = Nil.currencyBalance(address(this), tokenId1);
        {
            // scope for _token{0,1}, avoids stack too deep errors
            address _token0 = token0;
            address _token1 = token1;
            require(to != _token0 && to != _token1, "UniswapV2: INVALID_TO");
            if (amount0Out > 0) _safeTransfer(tokenId0, to, amount0Out);
            if (amount1Out > 0) _safeTransfer(tokenId1, to, amount1Out);
            balance0 = Nil.currencyBalance(address(this), tokenId0);
            balance1 = Nil.currencyBalance(address(this), tokenId1);
        }
        uint amount0In = balance0 > _reserve0 - amount0Out
            ? balance0 - (_reserve0 - amount0Out)
            : 0;
        uint amount1In = balance1 > _reserve1 - amount1Out
            ? balance1 - (_reserve1 - amount1Out)
            : 0;
        require(
            amount0In > 0 || amount1In > 0,
            "UniswapV2: INSUFFICIENT_INPUT_AMOUNT"
        );
        {
            // scope for reserve{0,1}Adjusted, avoids stack too deep errors
            uint balance0Adjusted = balance0.mul(1000).sub(amount0In.mul(3));
            uint balance1Adjusted = balance1.mul(1000).sub(amount1In.mul(3));
            require(
                balance0Adjusted.mul(balance1Adjusted) >=
                    uint(_reserve0).mul(_reserve1).mul(1000 ** 2),
                "UniswapV2: K"
            );
        }
        _update(balance0, balance1, _reserve0, _reserve1);
        emit Swap(msg.sender, amount0In, amount1In, amount0Out, amount1Out, to);
    }

    // force balances to match reserves
    function skim(address to) public lock {
        CurrencyId _tokenId0 = tokenId0; // gas savings
        CurrencyId _tokenId1 = tokenId1; // gas savings
        _safeTransfer(
            _tokenId0,
            to,
            Nil.currencyBalance(address(this), _tokenId0).sub(reserve0)
        );
        _safeTransfer(
            _tokenId1,
            to,
            Nil.currencyBalance(address(this), _tokenId1).sub(reserve1)
        );
    }

    // force reserves to match balances
    function sync() public lock {
        _update(
            Nil.currencyBalance(address(this), tokenId0),
            Nil.currencyBalance(address(this), tokenId1),
            reserve0,
            reserve1
        );
    }

    function token0Id() external view returns (CurrencyId) {
        return tokenId0;
    }

    function token1Id() external view returns (CurrencyId) {
        return tokenId1;
    }

    receive() external payable {}
}
