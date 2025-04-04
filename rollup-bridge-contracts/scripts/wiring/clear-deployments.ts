import { network } from 'hardhat';
import {
    ERC20TokenContract,
    L1MockContracts,
    loadL1MockConfig,
    loadL1NetworkConfig,
    saveL1MockConfig,
    saveL1NetworkConfig,
} from '../../deploy/config/config-helper';

// npx hardhat run scripts/wiring/clear-deployments.ts --network geth
export async function clearDeployments(networkName: string) {
    const config = loadL1NetworkConfig(networkName);

    // clear all deployed contract address under config
    config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerProxy = "";
    config.l1BridgeMessenger.l1BridgeMessengerContracts.l1BridgeMessengerImplementation = "";
    config.l1BridgeMessenger.l1BridgeMessengerContracts.proxyAdmin = "";

    config.l1BridgeRouter.l1BridgeRouterImplementation = "";
    config.l1BridgeRouter.l1BridgeRouterProxy = "";
    config.l1BridgeRouter.proxyAdmin = "";

    config.l1ERC20Bridge.l1ERC20BridgeImplementation = "";
    config.l1ERC20Bridge.l1ERC20BridgeProxy = "";
    config.l1ERC20Bridge.proxyAdmin = "";

    config.l1ETHBridge.l1ETHBridgeImplementation = "";
    config.l1ETHBridge.l1ETHBridgeProxy = "";
    config.l1ETHBridge.proxyAdmin = "";

    config.nilGasPriceOracle.nilGasPriceOracleContracts.nilGasPriceOracleImplementation = "";
    config.nilGasPriceOracle.nilGasPriceOracleContracts.nilGasPriceOracleProxy = "";
    config.nilGasPriceOracle.nilGasPriceOracleContracts.proxyAdmin = "";
    config.nilGasPriceOracle.nilGasPriceOracleDeployerConfig.proposerAddress = "";

    config.nilRollup.nilRollupContracts.nilRollupImplementation = "";
    config.nilRollup.nilRollupContracts.nilRollupProxy = "";
    config.nilRollup.nilRollupContracts.nilRollupImplementation = "";
    config.nilRollup.nilRollupContracts.nilVerifier = "";
    config.nilRollup.nilRollupContracts.proxyAdmin = "";
    config.nilRollup.nilRollupDeployerConfig.proposerAddress = "";

    config.l1CommonContracts.weth = "";
    config.l1DeployerConfig.admin = "";
    config.l1DeployerConfig.owner = "";

    saveL1NetworkConfig(networkName, config);

    const l1MockContracts: L1MockContracts = loadL1MockConfig(networkName);

    l1MockContracts.mockL2Bridge = "";
    const erc20Tokens: ERC20TokenContract[] = l1MockContracts.tokens;

    for (let erc20Token of erc20Tokens) {
        erc20Token.address = "";
    }

    l1MockContracts.tokens = erc20Tokens;

    const mockL2ERC20Tokens: ERC20TokenContract[] = l1MockContracts.mockL2Tokens;

    for (let mockErc20Token of mockL2ERC20Tokens) {
        mockErc20Token.address = "";
    }
    l1MockContracts.mockL2Tokens = mockL2ERC20Tokens;
    saveL1MockConfig(networkName, l1MockContracts);
}

async function main() {
    const networkName = network.name;
    await clearDeployments(networkName);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
