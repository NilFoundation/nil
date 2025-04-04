import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import {
    L1MockConfig,
    L1MockContracts,
    L1NetworkConfig,
    loadL1MockConfig,
    loadL1NetworkConfig,
    saveL1MockConfig,
    saveL1NetworkConfig,
} from '../../../config/config-helper';

export async function deployMockL2BridgeContract(networkName: string, deployer: any, deploy: any): Promise<void> {
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);
    try {
        const mockL2Bridge = await deploy('MockL2Bridge', {
            from: deployer,
            args: [],
            log: true,
        });

        console.log(`MockL2Bridge deployed to: ${mockL2Bridge.address}`);

        const l1MockContracts: L1MockContracts = loadL1MockConfig(networkName);

        l1MockContracts.mockL2Bridge = mockL2Bridge.address;
        saveL1MockConfig(networkName, l1MockContracts);
    } catch (error) {
        console.error('Error during deployment:', error);
    }

    // Save the updated config
    saveL1NetworkConfig(networkName, config);
}
