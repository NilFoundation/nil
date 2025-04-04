import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { run } from 'hardhat';
import {
    isValidAddress,
    isValidBytes32,
    archiveL1NetworkConfig,
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
} from '../config/config-helper';
import { verifyContractWithRetry } from '../common/proxy-contract-utils';

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

    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    // Check if NilVerifier is already deployed
    if (config.nilRollup.nilRollupContracts.nilVerifier && isValidAddress(config.nilRollup.nilRollupContracts.nilVerifier)) {
        console.log(`NilVerifier already deployed at: ${config.nilRollup.nilRollupContracts.nilVerifier}`);
        archiveL1NetworkConfig(networkName, config);
    }

    const nilVerifier = await deploy('NilVerifier', {
        from: deployer,
        args: [],
        log: true,
    });

    console.log('NilVerifier deployed to:', nilVerifier.address);
    config.nilRollup.nilRollupContracts.nilVerifier = nilVerifier.address;

    // Save the updated config
    saveL1NetworkConfig(networkName, config);

    // Skip verification if the network is local or anvil
    if (
        network.name !== 'local' &&
        network.name !== 'anvil' &&
        network.name !== 'geth'
    ) {
        try {
            await verifyContractWithRetry(nilVerifier.address, [], 6);
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
