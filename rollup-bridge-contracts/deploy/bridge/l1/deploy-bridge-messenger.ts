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
} from '../../config/config-helper';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../common/proxy-contract-utils';

// npx hardhat deploy --network sepolia --tags L1BridgeMessenger
// npx hardhat deploy --network geth --tags L1BridgeMessenger
const deployL1BridgeMessenger: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.l1Common.owner)) {
        throw new Error('Invalid nilRollupOwnerAddress in config');
    }
    if (!isValidAddress(config.l1Common.admin)) {
        throw new Error('Invalid defaultAdminAddress in config');
    }

    if (!isValidAddress(config.nilRollupConfig.nilRollupProxy)) {
        throw new Error('Invalid nilRollupProxy in config');
    }

    if (!config.l1BridgeMessengerConfig.maxProcessingTimeInEpochSeconds || config.l1BridgeMessengerConfig.maxProcessingTimeInEpochSeconds == 0) {
        throw new Error('Invalid maxProcessingTimeInEpochSeconds in l1BridgeMessengerConfig');
    }

    // Check if L1BridgeMessenger is already deployed
    if (config.l1BridgeMessengerConfig.l1BridgeMessengerProxy && isValidAddress(config.l1BridgeMessengerConfig.l1BridgeMessengerProxy)) {
        console.log(`l1BridgeMessenger already deployed at: ${config.l1BridgeMessengerConfig.l1BridgeMessengerProxy}`);
        archiveL1NetworkConfig(networkName, config);
    }

    try {
        // Deploy L1BridgeMessenger implementation
        const L1BridgeMessenger = await ethers.getContractFactory('L1BridgeMessenger');

        // proxy admin contract
        // deploys implementation contract (L1BridgeMessenger)
        // deploys ProxyContract
        // sets implementation contract address in the ProxyContract storage
        // initialize the contract
        // entire storage is owned by proxy contract
        const l1BridgeMessengerProxy = await upgrades.deployProxy(
            L1BridgeMessenger,
            [
                config.l1Common.owner, // _owner
                config.l1Common.admin, // _defaultAdmin
                config.nilRollupConfig.nilRollupProxy,
                config.l1BridgeMessengerConfig.maxProcessingTimeInEpochSeconds
            ],
            { initializer: 'initialize' },
        );

        console.log(`l1BridgeMessenger-Proxy deployed to: ${l1BridgeMessengerProxy.target}`);

        const l1BridgeMessengerProxyAddress = l1BridgeMessengerProxy.target;
        config.l1BridgeMessengerConfig.l1BridgeMessengerProxy = l1BridgeMessengerProxyAddress;

        // query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            l1BridgeMessengerProxyAddress,
        );
        config.l1BridgeMessengerConfig.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                l1BridgeMessengerProxyAddress,
            );
        config.l1BridgeMessengerConfig.l1BridgeMessengerImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = L1BridgeMessenger.attach(l1BridgeMessengerProxyAddress);

        // Save the updated config
        saveL1NetworkConfig(networkName, config);

        // check network and verify if its not geth or anvil
        // Skip verification if the network is local or anvil
        if (
            network.name !== 'local' &&
            network.name !== 'anvil' &&
            network.name !== 'geth'
        ) {
            try {
                await verifyContractWithRetry(l1BridgeMessengerProxyAddress, []);
            } catch (error) {
                console.error(
                    'L1BridgeMessenger Verification failed after retries:',
                    error,
                );
            }
        } else {
            console.log('Skipping verification on local or anvil network');
        }
    } catch (error) {
        console.error('Error during deployment:', error);
    }
};

export default deployL1BridgeMessenger;
deployL1BridgeMessenger.tags = ['L1BridgeMessenger'];
