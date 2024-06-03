// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract DelegateContract {
    uint256 public value;

    function setValue(uint256 _value) public {
        value = _value;
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}

contract ProxyContract {
    function setValue(address delegateAddress, uint256 _value) public {
        (bool success, ) = delegateAddress.delegatecall(
            abi.encodeWithSignature("setValue(uint256)", _value)
        );
        require(success, "Delegate call failed");
    }

    function getValue(address delegateAddress) public view returns (uint256) {
        (bool success, bytes memory result) = delegateAddress.staticcall(
            abi.encodeWithSignature("getValue()")
        );
        require(success, "Static call failed");
        return abi.decode(result, (uint256));
    }
}
