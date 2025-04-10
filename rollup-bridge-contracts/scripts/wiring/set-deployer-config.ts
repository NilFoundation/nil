import { network } from 'hardhat';
import {
    ERC20TokenContract,
    L1MockContracts,
    loadL1MockConfig,
    loadL1NetworkConfig,
    saveL1MockConfig,
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
    const networkName = network.name;
    await setDeployerConfig(networkName);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
