import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { run } from 'hardhat';
import {
    isValidAddress,
    isValidBytes32,
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
} from '../config/config-helper';
import { verifyContractWithRetry } from '../common/proxy-contract-utils';

export async function deployWETHTokenContract(networkName: string, deployer: any, deploy: any): Promise<void> {
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    const testWETH = await deploy('WETH', {
        from: deployer,
        args: [],
        log: true,
    });

    console.log('WETHToken deployed to:', testWETH.address);

    config.l1Common.weth = testWETH.address;

    // Skip verification if the network is local or anvil
    if (
        networkName !== 'local' &&
        networkName !== 'anvil' &&
        networkName !== 'geth'
    ) {
        try {
            await verifyContractWithRetry(testWETH.address, [], 6);
            console.log('WETHToken verified successfully');
        } catch (error) {
            console.error('WETHToken Verification failed:', error);
        }
    } else {
        console.log('Skipping verification on local or anvil network');
    }

    saveL1NetworkConfig(networkName, config);
}
