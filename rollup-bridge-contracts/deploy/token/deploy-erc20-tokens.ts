import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { run } from 'hardhat';
import {
    isValidAddress,
    isValidBytes32,
    L1NetworkConfig,
    loadL1NetworkConfig,
} from '../config/config-helper';
import { verifyContractWithRetry } from '../common/proxy-contract-utils';

// npx hardhat deploy --network sepolia --tags ERC20TokensDeploy
// npx hardhat deploy --network anvil --tags ERC20TokensDeploy
// npx hardhat deploy --network geth --tags ERC20TokensDeploy
const deployERC20Tokens: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, ethers, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;

    const { deployer } = await getNamedAccounts();

    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    const testERC20 = await deploy('TestERC20', {
        from: deployer,
        args: [],
        log: true,
    });

    console.log('ERC2Token deployed to:', testERC20.address);

    // Save the updated config
    //saveConfig(networkName, config);

    // Skip verification if the network is local or anvil
    if (
        network.name !== 'local' &&
        network.name !== 'anvil' &&
        network.name !== 'geth'
    ) {
        try {
            await verifyContractWithRetry(testERC20.address, [], 6);
            console.log('ERC20Token verified successfully');
        } catch (error) {
            console.error('ERC20Token Verification failed:', error);
        }
    } else {
        console.log('Skipping verification on local or anvil network');
    }
};

export default deployERC20Tokens;
deployERC20Tokens.tags = ['ERC20TokensDeploy'];
