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

        console.log(`smart-contract${deployerAccount.address} is on shard: ${deployerAccount.shardId} with balance: ${balance}`);

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        // save the nilMessageTree Address in the json config for l2
        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);

        const { address: l2EthBridgeVaultImplementationAddress, hash: l2EthBridgeVaultImplementationDeploymentTxHash } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: L2ETHBridgeVaultJson.default.bytecode,
            abi: L2ETHBridgeVaultJson.default.abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            //feeCredit: BigInt("19340180000000"),
        });

        console.log(`address from deployment is: ${l2EthBridgeVaultImplementationAddress}`);
        await waitTillCompleted(deployerAccount.client, l2EthBridgeVaultImplementationDeploymentTxHash);
        console.log("✅ Logic Contract deployed at:", l2EthBridgeVaultImplementationDeploymentTxHash);

        if (!l2EthBridgeVaultImplementationDeploymentTxHash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridgeVault Contract`);
        }

        if (!l2EthBridgeVaultImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridgeVault Contract`);
        }

        console.log(`NilMessageTree contract deployed at address: ${l2EthBridgeVaultImplementationAddress} and with transactionHash: ${l2EthBridgeVaultImplementationDeploymentTxHash}`);

        l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultImplementation = l2EthBridgeVaultImplementationAddress;

        const initData = encodeFunctionData({
            abi: L2ETHBridgeVaultJson.default.abi,
            functionName: "initialize",
            args: [l2NetworkConfig.l2CommonConfig.owner, l2NetworkConfig.l2CommonConfig.admin],
        });

        const { address: addressProxy, hash: hashProxy } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode,
            abi: TransparentUpgradeableProxy.default.abi,
            args: [l2EthBridgeVaultImplementationAddress, deployerAccount.address, initData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });
        await waitTillCompleted(deployerAccount.client, hashProxy);
        console.log("✅ Transparent Proxy Contract deployed at:", addressProxy);

        l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy = addressProxy;

        console.log("Waiting 5 seconds...");
        await new Promise((res) => setTimeout(res, 5000));

        const fetchImplementationCall = encodeFunctionData({
            abi: TransparentUpgradeableProxy.default.abi,
            functionName: "fetchImplementation",
            args: [],
        });

        const fetchImplementationResult = await deployerAccount.client.call({
            to: addressProxy,
            data: fetchImplementationCall,
            from: deployerAccount.address,
        }, "latest");

        console.log(`L2ETHBridgeVaultProxy has fetch-implementation-result: ${JSON.stringify(fetchImplementationResult)}`);

        const proxyImplementationAddress = decodeFunctionResult({
            abi: TransparentUpgradeableProxy.default.abi,
            functionName: "fetchImplementation",
            data: fetchImplementationResult.data,
        }) as string;

        console.log("✅ proxyImplementationAddress Address:", proxyImplementationAddress);

        const fetchAdminCall = encodeFunctionData({
            abi: TransparentUpgradeableProxy.default.abi,
            functionName: "fetchAdmin",
            args: [],
        });

        const adminResult = await deployerAccount.client.call({
            to: addressProxy,
            data: fetchAdminCall,
            from: deployerAccount.address,
        }, "latest");

        console.log(`L2ETHBridgeVaultProxy has admin-result: ${JSON.stringify(adminResult)}`);

        const proxyAdminAddress = decodeFunctionResult({
            abi: TransparentUpgradeableProxy.default.abi,
            functionName: "fetchAdmin",
            data: adminResult.data,
        }) as string;

        console.log("✅ ProxyAdmin Address:", proxyAdminAddress);

        l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.proxyAdmin = proxyAdminAddress;

        // Save the updated config
        saveNilNetworkConfig(networkName, l2NetworkConfig);
    });
