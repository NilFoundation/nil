pragma solidity ^0.8.0;

import "../../contracts/solidity/lib/Nil.sol";

contract ExternalIncrementer is NilBase {
    uint256 private value;

    constructor(uint256 initialValue) payable {
        value = initialValue;
    }

    function increment(uint256 _value) onlyExternal external {
       value += _value;
    }

    receive() external payable {}

    function get() public view returns(uint256) {
        return value;
    }

    function verifyExternal(uint256, bytes memory) external pure returns (bool) {
        return true;
    }
}
