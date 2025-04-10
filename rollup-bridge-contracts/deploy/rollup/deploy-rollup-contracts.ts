import { DeployFunction } from 'hardhat-deploy/types';
import { HardhatRuntimeEnvironment } from 'hardhat/types';
import { ethers, upgrades, run } from 'hardhat';
import {
    archiveL1NetworkConfig,
    isValidAddress,
    isValidBytes32,
    L1NetworkConfig,
    loadL1NetworkConfig,
    saveL1NetworkConfig,
    ZeroAddress,
} from '../config/config-helper';
import { BatchInfo, proposerRoleHash } from '../config/nil-types';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from '../common/proxy-contract-utils';

export async function deployRollupContracts(networkName: string, deployer: any, deploy: any): Promise<void> {
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    // Verify if the config object is not null and valid
    if (!config) {
        throw new Error(`Invalid NetworkConfig for network: ${networkName}`);
    }

    // Validate configuration parameters
    if (!isValidAddress(config.l1DeployerConfig.owner)) {
        throw new Error('Invalid nilRollupOwnerAddress in config');
    }
    if (!isValidAddress(config.l1DeployerConfig.admin)) {
        throw new Error('Invalid defaultAdminAddress in config');
    }
    if (!isValidAddress(config.nilRollup.nilRollupDeployerConfig.proposerAddress)) {
        throw new Error('Invalid proposerAddress in config');
    }
    if (!isValidBytes32(config.nilRollup.nilRollupInitConfig.genesisStateRoot)) {
        throw new Error('Invalid genesisStateRoot in config');
    }

    // Check if NilVerifier is already deployed
    if (config.nilRollup.nilRollupContracts.nilVerifier && isValidAddress(config.nilRollup.nilRollupContracts.nilVerifier)) {
        console.log(`NilVerifier already deployed at: ${config.nilRollup.nilRollupContracts.nilVerifier}`);
        archiveL1NetworkConfig(networkName, config);
    }

    console.log(`Deploying NilVerifier`);

    const nilVerifier = await deploy('NilVerifier', {
        from: deployer,
        args: [],
        log: true,
    });

    console.log('NilVerifier deployed to:', nilVerifier.address);
    config.nilRollup.nilRollupContracts.nilVerifier = nilVerifier.address;

    if (!isValidAddress(config.nilRollup.nilRollupContracts.nilVerifier)) {
        throw new Error('Invalid nilVerifier address in config');
    }

    const nilVerifierAddress = config.nilRollup.nilRollupContracts.nilVerifier;
    const l2ChainId = config.nilRollup.nilRollupInitConfig.l2ChainId;
    const proposerAddress = config.nilRollup.nilRollupDeployerConfig.proposerAddress;
    const ownerAddress = config.l1DeployerConfig.owner;
    const adminAddress = config.l1DeployerConfig.admin;

    try {
        // Deploy NilRollup implementation
        const NilRollup = await ethers.getContractFactory('NilRollup');

        const nilRollupProxy = await upgrades.deployProxy(
            NilRollup,
            [
                l2ChainId,
                ownerAddress, // _owner
                adminAddress, // _defaultAdmin
                nilVerifierAddress, // nilVerifier contract address
                proposerAddress, // proposer address
                config.nilRollup.nilRollupInitConfig.genesisStateRoot,
            ],
            { initializer: 'initialize' },
        );

        console.log(`NilRollup proxy deployed to: ${nilRollupProxy.target}`);

        const nilRollupProxyAddress = nilRollupProxy.target;
        config.nilRollup.nilRollupContracts.nilRollupProxy = nilRollupProxyAddress;

        // Query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(nilRollupProxyAddress);
        console.log(`ProxyAdmin for proxy: ${nilRollupProxyAddress} is: ${proxyAdminAddress}`);
        config.nilRollup.nilRollupContracts.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress = await upgrades.erc1967.getImplementationAddress(nilRollupProxyAddress);
        console.log(`Implementation address for proxy: ${nilRollupProxyAddress} is: ${implementationAddress}`);
        config.nilRollup.nilRollupContracts.nilRollupImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = NilRollup.attach(nilRollupProxyAddress);

        const storedL2ChainId = await nilRollup.l2ChainId();
        const storedOwnerAddress = await nilRollup.owner();
        const storedAdminAddress = await nilRollup.getRoleMember(await nilRollup.DEFAULT_ADMIN_ROLE(), 0);
        const storedNilVerifierAddress = await nilRollup.nilVerifierAddress();
        const storedProposerAddress = await nilRollup.getRoleMember(proposerRoleHash, 0);
        const storedGenesisStateRoot = await nilRollup
            .batchInfoRecords('GENESIS_BATCH_INDEX')
            .then((info: BatchInfo) => info.newStateRoot);

        if (storedL2ChainId.toString() !== l2ChainId.toString()) {
            throw new Error('l2ChainId mismatch');
        }
        if (storedOwnerAddress.toLowerCase() !== ownerAddress.toLowerCase()) {
            throw new Error('ownerAddress mismatch');
        }
        if (storedAdminAddress.toLowerCase() !== adminAddress.toLowerCase()) {
            throw new Error('adminAddress mismatch');
        }
        if (storedNilVerifierAddress.toLowerCase() !== nilVerifierAddress.toLowerCase()) {
            throw new Error('nilVerifierAddress mismatch');
        }
        if (storedProposerAddress.toLowerCase() !== proposerAddress.toLowerCase()) {
            throw new Error('proposerAddress mismatch');
        }
        if (storedGenesisStateRoot.toLowerCase() !== config.nilRollup.nilRollupInitConfig.genesisStateRoot.toLowerCase()) {
            throw new Error('genesisStateRoot mismatch');
        }

        // Save the updated config
        saveL1NetworkConfig(networkName, config);

        // Check network and verify if it's not geth or anvil
        if (networkName !== 'local' && networkName !== 'anvil' && networkName !== 'geth') {
            try {
                await verifyContractWithRetry(nilVerifier.address, []);
            } catch (error) {
                console.error('NilVerifier Verification failed after retries:', error);
            }

            try {
                await verifyContractWithRetry(nilRollupProxyAddress, []);
            } catch (error) {
                console.error('NilRollup Verification failed after retries:', error);
            }
        } else {
            console.log('Skipping verification on local or anvil network');
        }
    } catch (error) {
        console.error('Error during deployment:', error);
        process.exit(1);
    }
}