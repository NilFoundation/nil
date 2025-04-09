// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

/// @title INilAsync
/// @dev Interface to interact with Nil precompile for cross-shard async deployment and calls
interface INilAsync {
    /// @notice Deploys a contract asynchronously on a target shard
    /// @return deployed contract address
    function asyncDeploy(
        uint8 shard,
        address sender,
        address refundTo,
        uint256 value,
        uint256 gasLimit,
        uint256 gasPrice,
        bytes calldata bytecode,
        uint256 nonce
    ) external returns (address);

    /// @notice Performs an async cross-shard call
    /// @return success status of the call
    function asyncCall(
        uint8 shard,
        address sender,
        address to,
        uint256 value,
        uint256 gasLimit,
        uint256 gasPrice,
        bytes calldata data,
        uint256 nonce
    ) external returns (bool);
}
