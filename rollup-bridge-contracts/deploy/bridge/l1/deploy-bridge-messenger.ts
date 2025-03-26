import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import {
    archiveConfig,
    isValidAddress,
    isValidBytes32,
    loadConfig,
    NetworkConfig,
    saveConfig,
    ZeroAddress,
} from '../../config/config-helper';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../common/proxy-contract-utils';

// npx hardhat deploy --network sepolia --tags L1BridgeMessenger
// npx hardhat deploy --network anvil --tags L1BridgeMessenger
// npx hardhat deploy --network geth --tags L1BridgeMessenger
const deployL1BridgeMessenger: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    const config: NetworkConfig = loadConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollupConfig.owner)) {
        throw new Error('Invalid nilRollupOwnerAddress in config');
    }
    if (!isValidAddress(config.nilRollupConfig.admin)) {
        throw new Error('Invalid defaultAdminAddress in config');
    }

    // Check if L1BridgeMessenger is already deployed
    if (config.l1BridgeMessengerConfig.l1BridgeMessengerProxy && isValidAddress(config.l1BridgeMessengerConfig.l1BridgeMessengerProxy)) {
        console.log(`l1BridgeMessenger already deployed at: ${config.l1BridgeMessengerConfig.l1BridgeMessengerProxy}`);
        archiveConfig(networkName, config);
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
                config.l1BridgeMessengerConfig.owner, // _owner
                config.l1BridgeMessengerConfig.admin, // _defaultAdmin
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
        saveConfig(networkName, config);

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
