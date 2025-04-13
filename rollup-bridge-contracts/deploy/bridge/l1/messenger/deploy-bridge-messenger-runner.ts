import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import {
    archiveL1NetworkConfig,
    isValidAddress,
    isValidBytes32,
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
    ZeroAddress,
} from '../../../config/config-helper';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../../common/proxy-contract-utils';
import { deployL1BridgeMessengerContract } from './deploy-bridge-messenger-contract';

// npx hardhat deploy --network sepolia --tags L1BridgeMessenger
// npx hardhat deploy --network geth --tags L1BridgeMessenger
const deployL1BridgeMessenger: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    await deployL1BridgeMessengerContract(networkName);
};

export default deployL1BridgeMessenger;
deployL1BridgeMessenger.tags = ['L1BridgeMessenger'];
