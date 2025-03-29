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

// npx hardhat deploy --network sepolia --tags WETHTokenDeploy
// npx hardhat deploy --network anvil --tags WETHTokenDeploy
// npx hardhat deploy --network geth --tags WETHTokenDeploy
const deployWETHToken: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, ethers, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;

    const { deployer } = await getNamedAccounts();

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
        network.name !== 'local' &&
        network.name !== 'anvil' &&
        network.name !== 'geth'
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
};

export default deployWETHToken;
deployWETHToken.tags = ['WETHTokenDeploy'];
