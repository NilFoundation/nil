import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { run } from 'hardhat';
import {
    archiveConfig,
    isValidAddress,
    isValidBytes32,
    loadConfig,
    NetworkConfig,
    saveConfig,
} from './config/config-helper';

// npx hardhat deploy --network sepolia --tags NilVerifier
// npx hardhat deploy --network anvil --tags NilVerifier
// npx hardhat deploy --network geth --tags NilVerifier
const deployNilVerifier: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { deployments, getNamedAccounts, ethers, network } = hre;
    const { deploy } = deployments;
    const networkName = network.name;

    const { deployer } = await getNamedAccounts();

    const config: NetworkConfig = loadConfig(networkName);

    // Check if NilVerifier is already deployed
    if (config.nilVerifier && isValidAddress(config.nilVerifier)) {
        console.log(`NilVerifier already deployed at: ${config.nilVerifier}`);
        archiveConfig(networkName, config);
    }

    const nilVerifier = await deploy('NilVerifier', {
        from: deployer,
        args: [],
        log: true,
    });

    console.log('NilVerifier deployed to:', nilVerifier.address);
    config.nilVerifier = nilVerifier.address;

    // Save the updated config
    saveConfig(networkName, config);

    // Skip verification if the network is local or anvil
    if (
        network.name !== 'local' &&
        network.name !== 'anvil' &&
        network.name !== 'geth'
    ) {
        try {
            await run('verify:verify', {
                address: nilVerifier.address,
                constructorArguments: [],
            });
            console.log('NilVerifier verified successfully');
        } catch (error) {
            console.error('NilVerifier Verification failed:', error);
        }
    } else {
        console.log('Skipping verification on local or anvil network');
    }
};

export default deployNilVerifier;
deployNilVerifier.tags = ['NilVerifier'];
