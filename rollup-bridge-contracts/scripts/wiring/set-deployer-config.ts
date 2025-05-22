import {
    loadL1NetworkConfig,
    saveL1NetworkConfig,
} from '../../deploy/config/config-helper';

// npx hardhat run scripts/wiring/set-deployer-config.ts --network geth
export async function setDeployerConfig(networkName: string) {
    const config = loadL1NetworkConfig(networkName);

    // read .env and load variable
    const deployerAddress = process.env.GETH_WALLET_ADDRESS;

    if (!deployerAddress) {
        throw new Error(`DeployerAddress is not valid for network: ${networkName}`);
    }

    config.l1DeployerConfig.owner = deployerAddress;
    config.l1DeployerConfig.admin = deployerAddress;
    config.nilGasPriceOracle.nilGasPriceOracleDeployerConfig.proposerAddress = deployerAddress;
    config.nilRollup.nilRollupDeployerConfig.proposerAddress = deployerAddress;

    saveL1NetworkConfig(networkName, config);

}

async function main() {
    // Lazy import inside the function
    // @ts-ignore
    const { network } = await import('hardhat');
    const networkName = network.name;
    await setDeployerConfig(networkName);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
