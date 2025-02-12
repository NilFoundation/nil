// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import "../lib/Nil.sol";

contract L1BlockInfo {
    address public constant SELF_ADDRESS = address(0x222222222222222222222222222222222222);

    function setL1BlockInfo(
        uint64 _number,
        uint64 _timestamp,
        uint256 _baseFee,
        uint256 _blobBaseFee,
        bytes32 _hash
    ) external {
        require(msg.sender == SELF_ADDRESS, "setL1BlockInfo: only L1BlockInfo contract can be caller of this function");
        Nil.setConfigParam("l1block", abi.encode(Nil.ParamL1BlockInfo(_number, _timestamp, _baseFee, _blobBaseFee, _hash)));
    }
}