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
} from '../../../config/config-helper';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../../common/proxy-contract-utils';

export async function deployL1BridgeMessengerContract(networkName: string): Promise<void> {
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);
    // Validate configuration parameters
    if (!isValidAddress(config.l1DeployerConfig.owner)) {
        throw new Error('Invalid nilRollupOwnerAddress in config');
    }
    if (!isValidAddress(config.l1DeployerConfig.admin)) {
        throw new Error('Invalid defaultAdminAddress in config');
    }

    if (!isValidAddress(config.nilRollup.nilRollupContracts.nilRollupProxy)) {
        throw new Error('Invalid nilRollupProxy in config');
    }

    if (!config.l1BridgeMessenger.l1BridgeMessengerDeployerConfig.maxProcessingTimeInEpochSeconds ||
        config.l1BridgeMessenger.l1BridgeMessengerDeployerConfig.maxProcessingTimeInEpochSeconds == 0) {
        throw new Error('Invalid maxProcessingTimeInEpochSeconds in l1BridgeMessengerConfig');
    }

    try {
        // Deploy L1BridgeMessenger implementation
        const L1BridgeMessenger = await ethers.getContractFactory('L1BridgeMessenger');

        // Deploy proxy admin contract and initialize the proxy
        const l1BridgeMessengerProxy = await upgrades.deployProxy(
            L1BridgeMessenger,
            [
                config.l1DeployerConfig.owner, // _owner
                config.l1DeployerConfig.admin, // _defaultAdmin
                config.nilRollup.nilRollupContracts.nilRollupProxy,
                config.l1BridgeMessenger.l1BridgeMessengerDeployerConfig.maxProcessingTimeInEpochSeconds
            ],
            { initializer: 'initialize' },
        );

        console.log(`l1BridgeMessenger-Proxy deployed to: ${l1BridgeMessengerProxy.target}`);

        const l1BridgeMessengerProxyAddress = l1BridgeMessengerProxy.target;
        config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy = l1BridgeMessengerProxyAddress;

        // Query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            l1BridgeMessengerProxyAddress,
        );
        config.l1BridgeMessenger.l1BridgeMessengerContracts.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                l1BridgeMessengerProxyAddress,
            );
        config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Save the updated config
        saveL1NetworkConfig(networkName, config);

        // Check network and verify if it's not geth or anvil
        if (
            networkName !== 'local' &&
            networkName !== 'anvil' &&
            networkName !== 'geth'
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
        throw new Error(`Error while deploying L1BridgeMessenger on network: ${networkName} - ${error}`);
    }
}
