// SPDX-License-Identifier: MIT
pragma solidity 0.8.27;

import "@openzeppelin/contracts/proxy/transparent/TransparentUpgradeableProxy.sol";

contract CustomTransparentUpgradeableProxy is TransparentUpgradeableProxy {
    constructor(
        address _logic,
        address initialOwner,
        bytes memory _data
    )
        TransparentUpgradeableProxy(_logic, initialOwner, _data)
    { }

    function getAdmin() public view returns (address) {
        return _proxyAdmin();
    }
}
