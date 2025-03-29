import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import {
    archiveL1NetworkConfig,
    isValidAddress,
    isValidBytes32,
    L1NetworkConfig,
    L2NetworkConfig,
    loadL1NetworkConfig,
    loadNilNetworkConfig,
    saveL1NetworkConfig,
    saveNilNetworkConfig,
    ZeroAddress,
} from '../../config/config-helper';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../common/proxy-contract-utils';

// npx hardhat deploy --network geth --tags MockL2Bridge
const deployMockL2Bridge: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, ethers, network } = hre;
    const { deploy } = deployments;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);
    try {
        const mockL2Bridge = await deploy('MockL2Bridge', {
            from: deployer,
            args: [],
            log: true,
        });

        console.log(`MockL2Bridge deployed to: ${mockL2Bridge.address}`);

        config.l1Common.mockL2Bridge = mockL2Bridge.address;

    } catch (error) {
        console.error('Error during deployment:', error);
    }

    // Save the updated config
    saveL1NetworkConfig(networkName, config);

};

export default deployMockL2Bridge;
deployMockL2Bridge.tags = ['MockL2Bridge'];
