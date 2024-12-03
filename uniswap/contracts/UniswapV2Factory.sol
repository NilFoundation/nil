// SPDX-License-Identifier: UNLICENSED

pragma solidity ^0.8.0;

import "./interfaces/IUniswapV2Factory.sol";
import "./UniswapV2Pair.sol";
import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract UniswapV2Factory is IUniswapV2Factory {
    address public feeTo;
    address public feeToSetter;

    mapping(address => mapping(address => address)) public getPair;
    address[] public allPairs;

    event PairCreated(
        address indexed token0,
        address indexed token1,
        address pair,
        uint
    );

    constructor(address _feeToSetter) {
        feeToSetter = _feeToSetter;
    }

    function allPairsLength() public view returns (uint) {
        return allPairs.length;
    }

    function createPair(
        address tokenA,
        address tokenB,
        uint256 salt,
        uint256 shard
    ) public returns (address pair) {
        require(tokenA != tokenB, "UniswapV2: IDENTICAL(_ADDRESSES");
        (address token0, address token1) = tokenA < tokenB
            ? (tokenA, tokenB)
            : (tokenB, tokenA);
        require(token0 != address(0), "UniswapV2: ZERO_ADDRESS");
        require(
            getPair[token0][token1] == address(0),
            "UniswapV2: PAIR_EXISTS"
        );
        pair = address(deployPair(shard, salt));

        getPair[token0][token1] = pair;
        getPair[token1][token0] = pair;
        allPairs.push(pair);

        emit PairCreated(token0, token1, pair, allPairs.length);
    }

    function getTokenPair(
        address tokenA,
        address tokenB
    ) public view returns (address) {
        require(tokenA != tokenB, "UniswapV2: IDENTICAL(_ADDRESSES");
        (address token0, address token1) = tokenA < tokenB
            ? (tokenA, tokenB)
            : (tokenB, tokenA);
        require(token0 != address(0), "UniswapV2: ZERO_ADDRESS");
        return getPair[token0][token1];
    }

    function setFeeTo(address _feeTo) public {
        require(msg.sender == feeToSetter, "UniswapV2: FORBIDDEN");
        feeTo = _feeTo;
    }

    function setFeeToSetter(address _feeToSetter) public {
        require(msg.sender == feeToSetter, "UniswapV2: FORBIDDEN");
        feeToSetter = _feeToSetter;
    }

    function deployPair(
        uint256 shard,
        uint256 salt
    ) private returns (address deployedAddress) {
        bytes memory code = abi.encodePacked(
            type(UniswapV2Pair).creationCode,
            abi.encode(msg.sender)
        );
        address contractAddress = Nil.asyncDeploy(
            shard,
            msg.sender,
            0,
            code,
            salt
        );
        return contractAddress;
    }
}
