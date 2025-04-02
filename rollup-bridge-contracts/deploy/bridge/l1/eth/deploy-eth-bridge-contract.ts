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

export async function deployL1ETHBridgeContract(networkName: string): Promise<boolean> {
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);
    try {
        // Deploy L1ETHBridge implementation
        const L1ETHBridge = await ethers.getContractFactory('L1ETHBridge');

        // Deploy proxy admin contract and initialize the proxy
        const l1ETHBridgeProxy = await upgrades.deployProxy(
            L1ETHBridge,
            [
                config.l1Common.owner, // _owner
                config.l1Common.admin, // _defaultAdmin
                config.l1BridgeMessengerConfig.l1BridgeMessengerProxy,
                config.nilGasPriceOracleConfig.nilGasPriceOracleProxy
            ],
            { initializer: 'initialize' },
        );

        console.log(`l1ETHBridgeProxy deployed to: ${l1ETHBridgeProxy.target}`);

        const l1ETHBridgeProxyAddress = l1ETHBridgeProxy.target;
        config.l1ETHBridgeConfig.l1ETHBridgeProxy = l1ETHBridgeProxyAddress;

        // Query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            l1ETHBridgeProxyAddress,
        );
        config.l1ETHBridgeConfig.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                l1ETHBridgeProxyAddress,
            );
        config.l1ETHBridgeConfig.l1ETHBridgeImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        //const nilRollup = L1ETHBridge.attach(l1ETHBridgeProxyAddress);

        // Save the updated config
        saveL1NetworkConfig(networkName, config);

        // Check network and verify if it's not geth or anvil
        if (
            networkName !== 'local' &&
            networkName !== 'anvil' &&
            networkName !== 'geth'
        ) {
            try {
                await verifyContractWithRetry(l1ETHBridgeProxyAddress, []);
            } catch (error) {
                console.error(
                    'L1ETHBridge Verification failed after retries:',
                    error,
                );
            }
        } else {
            console.log('Skipping verification on local or anvil network');
            return true;
        }
    } catch (error) {
        console.error('Error during deployment:', error);
        throw new Error(`Error while deploying L1ETHBridge on network: ${networkName} - ${error}`);
    }

    return true;
}
