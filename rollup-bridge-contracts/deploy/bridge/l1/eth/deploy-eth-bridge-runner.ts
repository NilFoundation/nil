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
import { deployL1ETHBridgeContract } from './deploy-eth-bridge-contract';

// npx hardhat deploy --network sepolia --tags L1ETHBridge
// npx hardhat deploy --network geth --tags L1ETHBridge
const deployL1ETHBridge: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    await deployL1ETHBridgeContract(networkName);
};

export default deployL1ETHBridge;
deployL1ETHBridge.tags = ['L1ETHBridge'];
