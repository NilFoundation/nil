// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract DelegateContract {
    uint256 public value;

    constructor() payable {}

    function setValue(uint256 _value) public {
        value = _value;
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}

contract ProxyContract {
    constructor() payable {}

    function setValue(address delegateAddress, uint256 _value) public {
        (bool success, ) = delegateAddress.delegatecall(
            abi.encodeWithSignature("setValue(uint256)", _value)
        );
        require(success, "Delegate call failed");
    }

    function setValueStatic(address addr, uint256 _value) public view {
        (bool success, ) = addr.staticcall(
            abi.encodeWithSignature("setValue(uint256)", _value)
        );
        require(!success, "Static calls shouldn't write state");
    }

    function getValue(address delegateAddress) public returns (uint256) {
        (bool success, bytes memory result) = delegateAddress.delegatecall(
            abi.encodeWithSignature("getValue()")
        );
        require(success, "Static call failed");
        return abi.decode(result, (uint256));
    }
}
