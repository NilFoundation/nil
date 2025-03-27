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
} from './config/config-helper';
import { BatchInfo } from './config/nil-types';
import { sleepInMilliSeconds } from './common/helper-utils';
import { getProxyAdminAddressWithRetry, verifyContractWithRetry } from './common/proxy-contract-utils';

// npx hardhat deploy --network sepolia --tags NilRollup
// npx hardhat deploy --network anvil --tags NilRollup
// npx hardhat deploy --network geth --tags NilRollup
const deployNilRollup: DeployFunction = async function (
    hre: HardhatRuntimeEnvironment,
) {
    const { getNamedAccounts } = hre;
    const { deployer } = await getNamedAccounts();
    const networkName = network.name;
    const config: L1NetworkConfig = loadL1NetworkConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollupConfig.owner)) {
        throw new Error('Invalid nilRollupOwnerAddress in config');
    }
    if (!isValidAddress(config.nilRollupConfig.admin)) {
        throw new Error('Invalid defaultAdminAddress in config');
    }
    if (!isValidAddress(config.nilRollupConfig.proposerAddress)) {
        throw new Error('Invalid proposerAddress in config');
    }
    if (!isValidBytes32(config.nilRollupConfig.genesisStateRoot)) {
        throw new Error('Invalid genesisStateRoot in config');
    }

    if (!isValidAddress(config.nilRollupConfig.nilVerifier)) {
        throw new Error('Invalid nilVerifier address in config');
    }

    // Check if NilRollup is already deployed
    if (config.nilRollupConfig.nilRollupProxy && isValidAddress(config.nilRollupConfig.nilRollupProxy)) {
        console.log(`NilRollup already deployed at: ${config.nilRollupConfig.nilRollupProxy}`);
        archiveL1NetworkConfig(networkName, config);
    }

    const l2ChainId = config.nilRollupConfig.l2ChainId;

    try {
        // Deploy NilRollup implementation
        const NilRollup = await ethers.getContractFactory('NilRollup');

        // proxy admin contract
        // deploys implementation contract (NilRollup)
        // deploys ProxyContract
        // sets implementation contract address in the ProxyContract storage
        // initialize the contract
        // entire storage is owned by proxy contract
        const nilRollupProxy = await upgrades.deployProxy(
            NilRollup,
            [
                l2ChainId,
                config.nilRollupConfig.owner, // _owner
                config.nilRollupConfig.admin, // _defaultAdmin
                config.nilRollupConfig.nilVerifier, // nilVerifier contract address
                config.nilRollupConfig.proposerAddress, // proposer address
                config.nilRollupConfig.genesisStateRoot,
            ],
            { initializer: 'initialize' },
        );

        console.log(`NilRollup proxy deployed to: ${nilRollupProxy.target}`);

        const nilRollupProxyAddress = nilRollupProxy.target;
        config.nilRollupConfig.nilRollupProxy = nilRollupProxyAddress;

        // query proxyAdmin address and implementation address
        const proxyAdminAddress = await getProxyAdminAddressWithRetry(
            nilRollupProxyAddress,
        );
        config.nilRollupConfig.proxyAdmin = proxyAdminAddress;

        if (proxyAdminAddress === ZeroAddress) {
            throw new Error('Invalid proxy admin address');
        }

        const implementationAddress =
            await upgrades.erc1967.getImplementationAddress(
                nilRollupProxyAddress,
            );
        config.nilRollupConfig.nilRollupImplementation = implementationAddress;

        if (implementationAddress === ZeroAddress) {
            throw new Error('Invalid implementation address');
        }

        // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
        const nilRollup = NilRollup.attach(nilRollupProxyAddress);

        const storedL2ChainId = await nilRollup.l2ChainId();
        const storedOwnerAddress = await nilRollup.owner();
        const storedAdminAddress = await nilRollup.getRoleMember(
            await nilRollup.DEFAULT_ADMIN_ROLE(),
            0,
        );
        const storedNilVerifierAddress = await nilRollup.nilVerifierAddress();
        const storedProposerAddress = await nilRollup.getRoleMember(
            await nilRollup.PROPOSER_ROLE(),
            0,
        );
        const storedGenesisStateRoot = await nilRollup
            .batchInfoRecords('GENESIS_BATCH_INDEX')
            .then((info: BatchInfo) => info.newStateRoot);

        if (storedL2ChainId.toString() !== l2ChainId.toString()) {
            throw new Error('l2ChainId mismatch');
        }
        if (
            storedOwnerAddress.toLowerCase() !==
            config.nilRollupConfig.owner.toLowerCase()
        ) {
            throw new Error('ownerAddress mismatch');
        }
        if (
            storedAdminAddress.toLowerCase() !==
            config.nilRollupConfig.admin.toLowerCase()
        ) {
            throw new Error('adminAddress mismatch');
        }
        if (
            storedNilVerifierAddress.toLowerCase() !==
            config.nilRollupConfig.nilVerifier.toLowerCase()
        ) {
            throw new Error('nilVerifierAddress mismatch');
        }
        if (
            storedProposerAddress.toLowerCase() !==
            config.nilRollupConfig.proposerAddress.toLowerCase()
        ) {
            throw new Error('proposerAddress mismatch');
        }
        if (
            storedGenesisStateRoot.toLowerCase() !==
            config.nilRollupConfig.genesisStateRoot.toLowerCase()
        ) {
            throw new Error('genesisStateRoot mismatch');
        }

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
                await verifyContractWithRetry(nilRollupProxyAddress, []);
            } catch (error) {
                console.error(
                    'NilRollup Verification failed after retries:',
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

export default deployNilRollup;
deployNilRollup.tags = ['NilRollup'];
