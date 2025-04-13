import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { DeployFunction } from 'hardhat-deploy/types';
import { ethers, network, upgrades, run } from 'hardhat';
import {
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
    ZeroAddress,
} from '../../../config/config-helper';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../../common/proxy-contract-utils';

export async function deployL1ERC20BridgeContract(networkName: string): Promise<boolean> {
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    try {
        // Deploy L1ERC20Bridge implementation
        const L1ERC20Bridge = await ethers.getContractFactory('L1ERC20Bridge');

        // Deploy proxy admin contract and initialize the proxy
        const l1ERC20BridgeProxy = await upgrades.deployProxy(
            L1ERC20Bridge,
            [
                config.l1DeployerConfig.owner, // _owner
                config.l1DeployerConfig.admin, // _defaultAdmin
                config.l1CommonContracts.weth,
                config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy,
                config.nilGasPriceOracle.nilGasPriceOracleContracts.nilGasPriceOracleProxy
            ],
            { initializer: 'initialize' },
        );

        console.log(`l1ERC20BridgeProxy deployed to: ${l1ERC20BridgeProxy.target}`);

        const l1ERC20BridgeProxyAddress = l1ERC20BridgeProxy.target;
        config.l1ERC20Bridge.l1ERC20BridgeProxy = l1ERC20BridgeProxyAddress;

        // Query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            l1ERC20BridgeProxyAddress,
        );
        config.l1ERC20Bridge.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                l1ERC20BridgeProxyAddress,
            );
        config.l1ERC20Bridge.l1ERC20BridgeImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = L1ERC20Bridge.attach(l1ERC20BridgeProxyAddress);

        // Save the updated config
        saveL1NetworkConfig(networkName, config);

        // Check network and verify if it's not geth or anvil
        if (
            networkName !== 'local' &&
            networkName !== 'anvil' &&
            networkName !== 'geth'
        ) {
            try {
                await verifyContractWithRetry(l1ERC20BridgeProxyAddress, []);
            } catch (error) {
                console.error(
                    'L1ERC20Bridge Verification failed after retries:',
                    error,
                );
                return true;
            }
        } else {
            console.log('Skipping verification on local or anvil network');
            return true;
        }
    } catch (error) {
        console.error('Error during deployment:', error);
        throw new Error(`Error while deploying L1ERC20Bridge on network: ${networkName} - ${error}`);
    }
    return true;
}
