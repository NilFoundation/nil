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

// npx hardhat deploy-l2-eth-bridge-vault --networkname local
task("deploy-l2-eth-bridge-vault", "Deploys L2ETHBridgeVault contract on Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const L2ETHBridgeVaultJson = await import("../artifacts/contracts/bridge/l2/L2ETHBridgeVault.sol/L2ETHBridgeVault.json");
        const TransparentUpgradeableProxy = await import("../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");
        const ProxyAdmin = await import("../artifacts/node_modules/@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol/ProxyAdmin.json");

        if (!L2ETHBridgeVaultJson || !L2ETHBridgeVaultJson.default || !L2ETHBridgeVaultJson.default.abi || !L2ETHBridgeVaultJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridgeVault ABI`);
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

        const { tx: l2EthBridgeVaultImplementationDeploymentTx, address: l2EthBridgeVaultImplementationAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: L2ETHBridgeVaultJson.default.bytecode as `0x${string}`,
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            //feeCredit: BigInt("19340180000000"),
        });

        await waitTillCompleted(deployerAccount.client, l2EthBridgeVaultImplementationDeploymentTx.hash);

        if (!l2EthBridgeVaultImplementationDeploymentTx || !l2EthBridgeVaultImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridgeVault Contract`);
        }

        if (!l2EthBridgeVaultImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridgeVault Contract`);
        }

        console.log(`L2ETHBridgeVault contract deployed at address: ${l2EthBridgeVaultImplementationAddress} and with transactionHash: ${l2EthBridgeVaultImplementationDeploymentTx.hash}`);

        l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultImplementation = l2EthBridgeVaultImplementationAddress;

        const initData = encodeFunctionData({
            abi: L2ETHBridgeVaultJson.default.abi,
            functionName: "initialize",
            args: [l2NetworkConfig.l2CommonConfig.owner, l2NetworkConfig.l2CommonConfig.admin],
        });

        const { tx: proxyDeploymentTx, address: proxyAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string}`,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EthBridgeVaultImplementationAddress, deployerAccount.address, initData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });
        await waitTillCompleted(deployerAccount.client, proxyDeploymentTx.hash);
        l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy = proxyAddress;

        console.log(`L2ETHBridgeVaulProxy contract deployed at address: ${proxyAddress} and with transactionHash: ${proxyDeploymentTx.hash}`);

        const proxyContractInstance = getContract({
            client: deployerAccount.client,
            abi: TransparentUpgradeableProxy.default.abi,
            address: l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy as `0x${string}`,
        });

        console.log("Properties of proxyContractInsntace:", Object.keys(proxyContractInstance.read));

        const proxyAdminAddress = await proxyContractInstance.read.fetchAdmin([]);
        console.log("✅ ProxyAdmin Address:", proxyAdminAddress);
        l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.proxyAdmin = proxyAdminAddress as `0x${string}`;

        // Save the updated config
        saveNilNetworkConfig(networkName, l2NetworkConfig);

        const l2EthBridgeVaultProxyInstance = getContract({
            client: deployerAccount.client,
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            address: l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy as `0x${string}`
        });

        const ethAmountTracker = await l2EthBridgeVaultProxyInstance.read.ethAmountTracker([]);
        console.log("✅ ethAmountTracker:", ethAmountTracker);

        const l2ETHBridge = await l2EthBridgeVaultProxyInstance.read.l2ETHBridge([]);
        console.log(`l2ETHBridge queried from L2ETHBridgeVault is: ${l2ETHBridge}`);

        const l2ETHBridgeVaultOwner = await l2EthBridgeVaultProxyInstance.read.owner([]);
        console.log(`l2ETHBridgeVaultOwner queried from L2ETHBridgeVault is: ${l2ETHBridgeVaultOwner}`);

        const l2EthBridgeVaultImplementation = await l2EthBridgeVaultProxyInstance.read.getImplementation([]);
        console.log(`l2EthBridgeVaultImplementation queried from L2ETHBridgeVault is: ${l2EthBridgeVaultImplementation}`);

    });
