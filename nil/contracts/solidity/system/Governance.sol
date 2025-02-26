// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import "../lib/Nil.sol";

contract Governance is NilBase {
    address public constant SELF_ADDRESS =
        address(0x777777777777777777777777777777777777);

    function rollback(
        uint32 version,
        uint32 counter,
        uint32 patchLevel,
        uint64 mainBlockId,
        uint32 /*replayDepth*/,
        uint32 /*searchDepth*/
    ) external onlyExternal {
        Nil.rollback(
            version,
            counter,
            patchLevel,
            mainBlockId /*,
            replayDepth,
            searchDepth */
        );
    }

    bytes pubkey;

    constructor(bytes memory _pubkey) payable {
        pubkey = _pubkey;
    }

    function verifyExternal(
        uint256 hash,
        bytes memory authData
    ) external view returns (bool) {
        return Nil.validateSignature(pubkey, hash, authData);
    }
}
