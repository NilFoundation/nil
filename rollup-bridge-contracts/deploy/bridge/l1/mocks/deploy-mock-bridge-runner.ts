import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import {
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
} from '../../../config/config-helper';
import { deployMockL2BridgeContract } from './deploy-mock-bridge-contract';

// npx hardhat deploy --network geth --tags MockL2Bridge
const deployMockL2Bridge: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, ethers, network } = hre;
    const { deploy } = deployments;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    await deployMockL2BridgeContract(networkName, deploy, deployer);
};

export default deployMockL2Bridge;
deployMockL2Bridge.tags = ['MockL2Bridge'];
