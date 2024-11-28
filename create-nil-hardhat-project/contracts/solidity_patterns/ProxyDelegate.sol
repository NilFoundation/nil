// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Proxy {
    // slot for proxy.implementation variable
    bytes32 private constant _IMPLEMENTATION_SLOT = 0x5c60da1b41179947105cf0b285175f80782e9a8be68ed619b33c5b5d4de9dd9e;

    constructor(address implementation) {
        assembly {
            sstore(_IMPLEMENTATION_SLOT, implementation)
        }
    }

    function _implementation() internal view returns (address implementation) {
        assembly {
            implementation := sload(_IMPLEMENTATION_SLOT)
        }
    }

    function upgradeTo(address newImplementation) external {
        assembly {
            sstore(_IMPLEMENTATION_SLOT, newImplementation)
        }
    }

    fallback() external payable {
        address implementation = _implementation();
        assembly {
            calldatacopy(0, 0, calldatasize())
            let result := delegatecall(gas(), implementation, 0, calldatasize(), 0, 0)
            returndatacopy(0, 0, returndatasize())

            switch result
            case 0 { revert(0, returndatasize()) }
            default { return(0, returndatasize()) }
        }
    }
}

contract Delegate {
    // slot for proxy.value variable
    bytes32 private constant _VALUE_SLOT = 0x6f5d6a4da33e672a8e76a897ec60ff29e7b33e7d5850f7c4b13b8fb496d1063d;

    function setValue(uint256 newValue) external {
        assembly { sstore(_VALUE_SLOT, newValue) }
    }

    function getValue() external view returns (uint256 value) {
        assembly { value := sload(_VALUE_SLOT) }
    }
}

contract Delegate2 {
    // slot for proxy.value variable
    bytes32 private constant _VALUE_SLOT = 0x6f5d6a4da33e672a8e76a897ec60ff29e7b33e7d5850f7c4b13b8fb496d1063d;

    function setValue(uint256 newValue) external {
        uint256 v = newValue + 100;
        assembly { sstore(_VALUE_SLOT, v) }
    }

    function getValue() external view returns (uint256 value) {
        assembly { value := sload(_VALUE_SLOT) }
    }
}
