import type { Abi } from "abitype";
import { task } from "hardhat/config";
import {
    FaucetClient,
    HttpTransport,
    LocalECDSAKeySigner,
    PublicClient,
    SmartAccountV1,
    convertEthToWei,
    Transaction,
    generateRandomPrivateKey,
    waitTillCompleted,
    getContract,
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { decodeFunctionResult, encodeFunctionData } from "viem";

// npx hardhat deploy-nil-eth-bridge --networkname local
task("deploy-nil-eth-bridge", "Deploys NilETHBridge contract on Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const NilETHBridgeJson = await import("../artifacts/contracts/bridge/l2/NilETHBridge.sol/NilETHBridge.json");
        const TransparentUpgradeableProxy = await import("../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");
        const ProxyAdmin = await import("../artifacts/node_modules/@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol/ProxyAdmin.json");

        if (!NilETHBridgeJson || !NilETHBridgeJson.default || !NilETHBridgeJson.default.abi || !NilETHBridgeJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridge ABI`);
        }

        const networkName = taskArgs.networkname;
        console.log(`Running task on network: ${networkName}`);

        const deployerAccount = await loadNilSmartAccount();

        if (!deployerAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const balance = await deployerAccount.getBalance();

        console.log(`deployer-smart-account: ${deployerAccount.address} is on shard: ${deployerAccount.shardId} with balance: ${balance}`);

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        // save the nilMessageTree Address in the json config for l2
        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);

        const { tx: l2EthBridgeImplementationDeploymentTx, address: l2EthBridgeImplementationAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: NilETHBridgeJson.default.bytecode as `0x${string}`,
            abi: NilETHBridgeJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: BigInt("19340180000000"),
        });

        await waitTillCompleted(deployerAccount.client, l2EthBridgeImplementationDeploymentTx.hash, {
            waitTillMainShard: true
        });

        if (!l2EthBridgeImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridge Contract`);
        }

        if (!l2EthBridgeImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridge Contract`);
        }

        console.log(`L2ETHBridge contract deployed at address: ${l2EthBridgeImplementationAddress} and with transactionHash: ${l2EthBridgeImplementationDeploymentTx.hash}`);

        l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeImplementation = l2EthBridgeImplementationAddress;

        const initData = encodeFunctionData({
            abi: NilETHBridgeJson.default.abi,
            functionName: "initialize",
            args: [l2NetworkConfig.l2CommonConfig.owner,
            l2NetworkConfig.l2CommonConfig.admin,
            l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy,
            l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy
            ],
        });
        const { tx: l2EthBridgeProxyDeploymentTx, address: l2EthBridgeProxyAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string}`,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EthBridgeImplementationAddress, deployerAccount.address, initData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.01),
        });
        await waitTillCompleted(deployerAccount.client, l2EthBridgeProxyDeploymentTx.hash, {
            waitTillMainShard: true
        });
        console.log("✅ Transparent Proxy Contract deployed at:", l2EthBridgeProxyAddress);

        l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy = l2EthBridgeProxyAddress;

        const l2EthBridgeProxyInstance = getContract({
            client: deployerAccount.client,
            abi: NilETHBridgeJson.default.abi as Abi,
            address: l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy as `0x${string}`
        });

        const l2ETHBridgeVaultOwner = await l2EthBridgeProxyInstance.read.owner([]);
        console.log(`l2ETHBridgeVaultOwner queried from L2ETHBridgeVault is: ${l2ETHBridgeVaultOwner}`);

        const l2EthBridgeVaultImplementation = await l2EthBridgeProxyInstance.read.getImplementation([]);
        console.log(`l2EthBridgeVaultImplementation queried from L2ETHBridgeVault is: ${l2EthBridgeVaultImplementation}`);
    });
