// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../../../contracts/solidity/lib/Nil.sol";

contract Callee {
    int32 value;
    event LogInfo(string message);

    constructor() payable {}

    function add(int32 val) public payable returns (int32) {
        emit LogInfo("execution started");
        require(val != 0, "Value must be non-zero");
        value += val;
        return value;
    }
}

contract Caller is NilBounceable {
    using Nil for address;

    string last_bounce_err;

    constructor() payable {}

    function call(address dst, int32 val) public payable {
        dst.asyncCall(
            address(0), // refundTo
            address(0), // bounceTo
            gasleft() * tx.gasprice, // gas
            Nil.FORWARD_NONE, // forwardKind
            false,
            msg.value,
            abi.encodeWithSignature("add(int32)", val)
        );
    }

    function sendMessage(bytes calldata message) public payable {
        Nil.sendMessage(message);
    }

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
    }

    function bounce(string calldata err) onlyInternal override external payable {
        last_bounce_err = err;
    }

    function get_bounce_err() public view returns (string memory) {
        return last_bounce_err;
    }
}
