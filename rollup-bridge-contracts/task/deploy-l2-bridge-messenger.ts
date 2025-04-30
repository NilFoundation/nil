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
        console.log(`Running task on network: ${networkName}`);

        const deployerAccount = await loadNilSmartAccount();

        if (!deployerAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const balance = await deployerAccount.getBalance();

        console.log(`smart-contract: ${deployerAccount.address} is on shard: ${deployerAccount.shardId} with balance: ${balance}`);

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        // save the L2BridgeMessenger Address in the json config for l2
        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);



        const { tx: nilMessengerImplementationDeploymentTx, address: nilMessengerImplementationAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: L2BridgeMessengerJson.default.bytecode as `0x${string}`,
            abi: L2BridgeMessengerJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: BigInt("19340180000000"),
        });

        console.log(`L2BridgeMessenger contractis deployed at: ${nilMessengerImplementationAddress} with transactionHash: ${nilMessengerImplementationDeploymentTx.hash}`);

        if (!nilMessengerImplementationDeploymentTx || !nilMessengerImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2BridgeMessenger Contract`);
        }

        if (!nilMessengerImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2BridgeMessenger Contract`);
        }

        console.log(`L2BridgeMessenger contract deployed at address: ${nilMessengerImplementationAddress} and with transactionHash: ${nilMessengerImplementationAddress}`);

        l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerImplementation = nilMessengerImplementationAddress;

        const initData = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi,
            functionName: "initialize",
            args: [l2NetworkConfig.l2CommonConfig.owner, l2NetworkConfig.l2CommonConfig.admin,
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
        await waitTillCompleted(deployerAccount.client, proxyDeploymentTx.hash);
        l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy = proxyAddress;

        console.log("Waiting 5 seconds...");
        await new Promise((res) => setTimeout(res, 5000));


        const messengerContractInstance = getContract({
            client: deployerAccount.client,
            abi: L2BridgeMessengerJson.default.abi as Abi,
            address: proxyAddress,
        });

        console.log("Properties of messengerContractInstance:", Object.keys(messengerContractInstance.read));

        const implementationAddress = await messengerContractInstance.read.getImplementation([]);
        console.log(`implementationAddress from MyLogic is: ${implementationAddress}`);
    });
