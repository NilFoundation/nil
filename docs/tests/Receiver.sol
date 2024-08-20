// SPDX-License-Identifier: MIT

//startReceiverContract
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
contract Receiver {
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
//endReceiverContract
