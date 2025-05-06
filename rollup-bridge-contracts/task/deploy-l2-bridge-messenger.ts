import type { Abi } from "abitype";
import { task } from "hardhat/config";
import {
    FaucetClient,
    HttpTransport,
    LocalECDSAKeySigner,
    PublicClient,
    SmartAccountV1,
    getContract,
    convertEthToWei,
    Transaction,
    generateRandomPrivateKey,
    waitTillCompleted,
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { getCheckSummedAddress, validateAddress } from "../scripts/utils/validate-config";
import { decodeFunctionResult, encodeFunctionData } from "viem";

// npx hardhat deploy-l2-bridge-messenger --networkname local
task("deploy-l2-bridge-messenger", "Deploys L2BridgeMessenger contract on Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const L2BridgeMessengerJson = await import("../artifacts/contracts/bridge/l2/L2BridgeMessenger.sol/L2BridgeMessenger.json");
        const TransparentUpgradeableProxy = await import("../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");
        const ProxyAdmin = await import("../artifacts/node_modules/@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol/ProxyAdmin.json");

        if (!L2BridgeMessengerJson || !L2BridgeMessengerJson.default || !L2BridgeMessengerJson.default.abi || !L2BridgeMessengerJson.default.bytecode) {
            throw Error(`Invalid L2BridgeMessengerJson ABI`);
        }

        const networkName = taskArgs.networkname;
        const deployerAccount = await loadNilSmartAccount();

        if (!deployerAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const balance = await deployerAccount.getBalance();

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        // save the L2BridgeMessenger Address in the json config for l2
        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);

        validateAddress(l2NetworkConfig.l2CommonConfig.owner, "l2CommonConfig.owner");
        validateAddress(l2NetworkConfig.l2CommonConfig.admin, "l2CommonConfig.admin");
        validateAddress(
            l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerDeployerConfig.relayerAddress,
            "l2BridgeMessengerDeployerConfig.relayerAddress"
        );
        validateAddress(
            l2NetworkConfig.nilMessageTreeConfig.nilMessageTreeContracts.nilMessageTreeImplementationAddress,
            "nilMessageTreeContracts.nilMessageTreeImplementationAddress"
        );

        if (
            !l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerDeployerConfig.messageExpiryDeltaValue ||
            typeof l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerDeployerConfig.messageExpiryDeltaValue !== "number" ||
            l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerDeployerConfig.messageExpiryDeltaValue <= 0
        ) {
            throw new Error("Invalid configuration: l2BridgeMessengerDeployerConfig.messageExpiryDeltaValue must be a positive number.");
        }


        const { tx: nilMessengerImplementationDeploymentTx, address: nilMessengerImplementationAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: L2BridgeMessengerJson.default.bytecode as `0x${string}`,
            abi: L2BridgeMessengerJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await waitTillCompleted(deployerAccount.client, nilMessengerImplementationDeploymentTx.hash, {
            waitTillMainShard: true
        });

        if (!nilMessengerImplementationDeploymentTx || !nilMessengerImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2BridgeMessenger Contract`);
        }

        if (!nilMessengerImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2BridgeMessenger Contract`);
        }

        l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerImplementation = getCheckSummedAddress(nilMessengerImplementationAddress);

        const initData = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi,
            functionName: "initialize",
            args: [l2NetworkConfig.l2CommonConfig.owner,
            l2NetworkConfig.l2CommonConfig.admin,
            l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerDeployerConfig.relayerAddress,
            l2NetworkConfig.nilMessageTreeConfig.nilMessageTreeContracts.nilMessageTreeImplementationAddress,
            l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerDeployerConfig.messageExpiryDeltaValue],
        });

        const { tx: proxyDeploymentTx, address: proxyAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string}`,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [nilMessengerImplementationAddress, deployerAccount.address, initData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await waitTillCompleted(deployerAccount.client, proxyDeploymentTx.hash, {
            waitTillMainShard: true
        });

        l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy = getCheckSummedAddress(proxyAddress);

        const proxyContractInstance = getContract({
            client: deployerAccount.client,
            abi: TransparentUpgradeableProxy.default.abi,
            address: proxyAddress as `0x${string}`,
        });

        const proxyAdminAddress = await proxyContractInstance.read.fetchAdmin([]);
        l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.proxyAdmin = proxyAdminAddress as `0x${string}`;

        // Save the updated config
        saveNilNetworkConfig(networkName, l2NetworkConfig);
    });
