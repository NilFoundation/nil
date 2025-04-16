// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../../contracts/solidity/lib/Nil.sol";

contract Callee {
    int32 value;

    constructor() payable {}

    function add(int32 val) public payable returns (int32) {
        Nil.log("execution started");
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
            msg.value,
            abi.encodeWithSignature("add(int32)", val)
        );
    }

    function asyncCall(
        address dst,
        address refundTo,
        address bounceTo,
        uint feeCredit,
        uint8 forwardKind,
        uint value,
        bytes memory callData
    ) public payable {
        Nil.asyncCall(dst, refundTo, bounceTo, feeCredit, forwardKind, value, callData);
    }

    function verifyExternal(
        uint256,
        bytes calldata
    ) external pure returns (bool) {
        return true;
    }

    function bounce(
        string calldata err
    ) external payable override onlyInternal {
        last_bounce_err = err;
    }

    function get_bounce_err() public view returns (string memory) {
        return last_bounce_err;
    }
}
