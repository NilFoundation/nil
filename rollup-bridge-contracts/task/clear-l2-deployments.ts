import { task } from "hardhat/config";
import "dotenv/config";
import { EnshrinedToken, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";

// npx hardhat clear-l2-deployments --networkname local
task("clear-l2-deployments", "Clears L2DeploymentConfig entries in nil-deployment-config.json")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {
        const networkName = taskArgs.networkname;
        console.log(`Running task on network: ${networkName}`);

        const config = loadNilNetworkConfig(networkName);

        config.l2CommonConfig.admin = "";
        config.l2CommonConfig.owner = "";
        config.l2CommonConfig.mockL1Bridge = "";
        config.l2CommonConfig.relayer = "";

        // clear all deployed contract address under config
        config.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerImplementation = "";
        config.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy = "";
        config.l2BridgeMessengerConfig.l2BridgeMessengerContracts.proxyAdmin = "";

        config.l2CommonConfig.mockL1Bridge = "";

        config.l2TestConfig.erc20TestEventData.messageHash = "";
        config.l2TestConfig.ethTestEventData.messageHash = "";

        config.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeImplementation = "";
        config.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy = "";
        config.l2ETHBridgeConfig.l2ETHBridgeContracts.proxyAdmin = "";

        config.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultImplementation = "";
        config.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy = "";
        config.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.proxyAdmin = "";

        config.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeImplementation = "";
        config.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy = "";
        config.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.proxyAdmin = "";

        config.nilMessageTreeConfig.nilMessageTreeContracts.nilMessageTreeImplementationAddress = "";

        const enshrinedTokens: EnshrinedToken[] = config.l2CommonConfig.tokens;

        for (let enshrinedToken of enshrinedTokens) {
            enshrinedToken.address = "";
        }

        config.l2CommonConfig.tokens = enshrinedTokens;

        saveNilNetworkConfig(networkName, config);
    });
