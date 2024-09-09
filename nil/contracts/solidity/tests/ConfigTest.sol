// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.9;

import "../../../../smart-contracts/contracts/Nil.sol";

contract ConfigTest is NilBase {

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    function testValidatorsEqual(Nil.ParamValidators memory inputValidators) public {
        Nil.ParamValidators memory realValidators = Nil.getValidators();
        require(inputValidators.list.length == realValidators.list.length, "Lengths are not equal");
        for (uint i = 0; i < inputValidators.list.length; i++) {
            bytes32 a = keccak256(abi.encodePacked(inputValidators.list[i].PublicKey));
            bytes32 b = keccak256(abi.encodePacked(realValidators.list[i].PublicKey));
            require(a == b, "Public keys are not equal");
            require(inputValidators.list[i].WithdrawalAddress == realValidators.list[i].WithdrawalAddress, "Withdraw addresses are not equal");
        }
    }

    function setValidators(Nil.ParamValidators memory inputValidators) public {
        Nil.setValidators(inputValidators);
    }

    function testParamGasPriceEqual(Nil.ParamGasPrice memory param) public {
        Nil.ParamGasPrice memory realParam = Nil.getParamGasPrice();
        require(param.gasPriceScale == realParam.gasPriceScale, "Gas price scales are not equal");
    }

    function setParamGasPrice(Nil.ParamGasPrice memory param) public {
        Nil.setParamGasPrice(param);
    }
}
