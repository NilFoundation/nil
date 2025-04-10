import { ethers, network, upgrades, run } from 'hardhat';
import {
    loadL1NetworkConfig,
    saveL1NetworkConfig,
    ZeroAddress,
} from '../../../config/config-helper';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../../../common/proxy-contract-utils';

export async function deployNilGasPriceOracleContract(networkName: string): Promise<void> {
    const config = loadL1NetworkConfig(networkName);
    try {
        // Deploy NilGasPriceOracle implementation
        const NilGasPriceOracle = await ethers.getContractFactory('NilGasPriceOracle');

        // Deploy proxy admin contract and initialize the proxy
        const nilGasPriceOracleProxy = await upgrades.deployProxy(
            NilGasPriceOracle,
            [
                config.l1DeployerConfig.owner, // _owner
                config.l1DeployerConfig.admin, // _defaultAdmin
                config.nilGasPriceOracle.nilGasPriceOracleDeployerConfig.proposerAddress,
                config.nilGasPriceOracle.nilGasPriceOracleDeployerConfig.nilGasPriceOracleMaxFeePerGas,
                config.nilGasPriceOracle.nilGasPriceOracleDeployerConfig.nilGasPriceOracleMaxPriorityFeePerGas
            ],
            { initializer: 'initialize' },
        );

        console.log(`nilGasPriceOracleProxy deployed to: ${nilGasPriceOracleProxy.target}`);

        const nilGasPriceOracleProxyAddress = nilGasPriceOracleProxy.target;
        config.nilGasPriceOracle.nilGasPriceOracleContracts.nilGasPriceOracleProxy = nilGasPriceOracleProxyAddress;

        // Query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            nilGasPriceOracleProxyAddress,
        );
        config.nilGasPriceOracle.nilGasPriceOracleContracts.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                nilGasPriceOracleProxyAddress,
            );
        config.nilGasPriceOracle.nilGasPriceOracleContracts.nilGasPriceOracleImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = NilGasPriceOracle.attach(nilGasPriceOracleProxyAddress);

        // Save the updated config
        saveL1NetworkConfig(networkName, config);

        // Check network and verify if it's not geth or anvil
        if (
            networkName !== 'local' &&
            networkName !== 'anvil' &&
            networkName !== 'geth'
        ) {
            try {
                await verifyContractWithRetry(nilGasPriceOracleProxyAddress, []);
            } catch (error) {
                console.error(
                    'NilGasPriceOracleProxy Verification failed after retries:',
                    error,
                );
            }
        } else {
            console.log('Skipping verification on local or anvil network');
        }
    } catch (error) {
        console.error('Error during deployment:', error);
        throw new Error(`Error while deploying NilGasPriceOracle on network: ${networkName} - ${error}`);
    }
}
