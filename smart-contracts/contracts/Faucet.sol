// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Nil.sol";
import "./SmartAccount.sol";

contract Faucet {
    uint256 private constant WITHDRAW_PER_TIMEOUT_LIMIT = 10**16;
    uint256 private constant TIMEOUT = 200; // 200 blocks

    struct LimitInfo {
        uint prevT;
        uint prevLimit;
    }
    mapping(address => LimitInfo) private limits;

//         Limit
//           ^
//           |
// max_limit +---------+  +----+        +----+     +---
//           |         | /     |       /     |    /
//           |         |/      |      /      |   /
//           |                 |  /| /       |  /
//           |                 | / |/        | /
//           |                 |/            |/
//           +-------------------------------+-----+---> Time
//                                              ^
//                                           timeout
//
//           k = max_limit / timeout
//           current_limit = min(prev_limit + delta_t * k, max_limit)
    function acquire(address addr, uint256 value) private returns (uint256) {
        LimitInfo memory limitInfo = limits[addr];

        uint256 currentT = block.number;
        uint256 currentLimit;
        if (limitInfo.prevT == 0) {
            currentLimit = WITHDRAW_PER_TIMEOUT_LIMIT;
        } else {
            uint256 deltaT = currentT - limitInfo.prevT;
            currentLimit = limitInfo.prevLimit + (WITHDRAW_PER_TIMEOUT_LIMIT / TIMEOUT) * deltaT;
            if (currentLimit > WITHDRAW_PER_TIMEOUT_LIMIT) {
                currentLimit = WITHDRAW_PER_TIMEOUT_LIMIT;
            }
        }
        uint256 acquired = value;
        if (value > currentLimit) {
            acquired = currentLimit;
        }

        limits[addr] = LimitInfo(currentT, currentLimit - acquired);

        return acquired;
    }

    event Deploy(address addr);
    event Send(address addr, uint256 value);

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    function withdrawTo(address payable addr, uint256 value) public {
        value = acquire(addr, value);

        bytes memory callData;
        uint feeCredit = 100_000 * tx.gasprice;
        Nil.asyncCall(
            addr,
            address(this) /* refundTo */,
            address(this) /* bounceTo */,
            feeCredit,
            Nil.FORWARD_NONE,
            value,
            callData);
        emit Send(addr, value);
    }

    function deploy(
        uint shardId,
        bytes memory code,
        bytes32 salt,
        uint256 value
    ) external returns (address) {
        address addr = Nil.asyncDeploy(
            shardId,
            address(this),
            value,
            code,
            uint256(salt)
        );
        emit Deploy(addr);
        emit Send(addr, value);
        return addr;
    }
}

contract FaucetToken is NilTokenBase {
    event Send(address addr, uint256 value);

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    function withdrawTo(address payable addr, uint256 value) public {
        mintTokenInternal(value);
        sendTokenInternal(addr, getTokenId(), value);

        emit Send(addr, value);
    }
}
