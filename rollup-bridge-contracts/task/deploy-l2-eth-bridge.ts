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

// npx hardhat deploy-l2-eth-bridge --networkname local
task("deploy-l2-eth-bridge", "Deploys L2ETHBridge contract on Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const L2ETHBridgeJson = await import("../artifacts/contracts/bridge/l2/L2ETHBridge.sol/L2ETHBridge.json");
        const TransparentUpgradeableProxy = await import("../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");
        const ProxyAdmin = await import("../artifacts/node_modules/@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol/ProxyAdmin.json");

        if (!L2ETHBridgeJson || !L2ETHBridgeJson.default || !L2ETHBridgeJson.default.abi || !L2ETHBridgeJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridge ABI`);
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

        const { tx: l2EthBridgeImplementationDeploymentTx, address: l2EthBridgeImplementationAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: L2ETHBridgeJson.default.bytecode as `0x${string}`,
            abi: L2ETHBridgeJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: BigInt("19340180000000"),
        });

        console.log(`address from deployment is: ${l2EthBridgeImplementationAddress} and transactionHash: ${l2EthBridgeImplementationDeploymentTx.hash}`);
        await waitTillCompleted(deployerAccount.client, l2EthBridgeImplementationDeploymentTx.hash);

        if (!l2EthBridgeImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridge Contract`);
        }

        if (!l2EthBridgeImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridge Contract`);
        }

        console.log(`NilMessageTree contract deployed at address: ${l2EthBridgeImplementationAddress} and with transactionHash: ${l2EthBridgeImplementationDeploymentTx.hash}`);

        l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeImplementation = l2EthBridgeImplementationAddress;

        const initData = encodeFunctionData({
            abi: L2ETHBridgeJson.default.abi,
            functionName: "initialize",
            args: [l2NetworkConfig.l2CommonConfig.owner, l2NetworkConfig.l2CommonConfig.admin,
            l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy,
            l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy],
        });
        const { tx: l2EthBridgeProxyDeploymentTx, address: l2EthBridgeProxyAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string}`,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EthBridgeImplementationAddress, deployerAccount.address, initData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });
        await waitTillCompleted(deployerAccount.client, l2EthBridgeProxyDeploymentTx.hash);
        console.log("✅ Transparent Proxy Contract deployed at:", l2EthBridgeProxyAddress);

        l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy = l2EthBridgeProxyAddress;

        console.log("Waiting 5 seconds...");
        await new Promise((res) => setTimeout(res, 5000));

        const fetchImplementationCall = encodeFunctionData({
            abi: TransparentUpgradeableProxy.default.abi,
            functionName: "fetchImplementation",
            args: [],
        });

        const fetchImplementationResult = await deployerAccount.client.call({
            to: l2EthBridgeProxyAddress,
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
            to: l2EthBridgeProxyAddress,
            data: fetchAdminCall,
            from: deployerAccount.address,
        }, "latest");

        console.log(`L2ETHBridgeProxy has admin-result: ${JSON.stringify(adminResult)}`);

        const proxyAdminAddress = decodeFunctionResult({
            abi: TransparentUpgradeableProxy.default.abi,
            functionName: "fetchAdmin",
            data: adminResult.data,
        }) as string;

        console.log("✅ ProxyAdmin Address:", proxyAdminAddress);

        l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.proxyAdmin = proxyAdminAddress;

        // Save the updated config
        saveNilNetworkConfig(networkName, l2NetworkConfig);
    });
