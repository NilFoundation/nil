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
    getContract
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { decodeFunctionResult, encodeFunctionData } from "viem";
import { getCheckSummedAddress, validateAddress } from "../scripts/utils/validate-config";

// npx hardhat deploy-l2-enshrined-token-bridge --networkname local
task("deploy-l2-enshrined-token-bridge", "Deploys L2EnshrinedTokenBridge contract on Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const L2EnshrinedTokenBridgeJson = await import("../artifacts/contracts/bridge/l2/L2EnshrinedTokenBridge.sol/L2EnshrinedTokenBridge.json");
        const TransparentUpgradeableProxy = await import("../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");
        const ProxyAdmin = await import("../artifacts/node_modules/@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol/ProxyAdmin.json");

        if (!L2EnshrinedTokenBridgeJson || !L2EnshrinedTokenBridgeJson.default || !L2EnshrinedTokenBridgeJson.default.abi || !L2EnshrinedTokenBridgeJson.default.bytecode) {
            throw Error(`Invalid L2EnshrinedTokenBridge ABI`);
        }

        const networkName = taskArgs.networkname;
        console.log(`Running task on network: ${networkName}`);

        const deployerAccount = await loadNilSmartAccount();

        if (!deployerAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const balance = await deployerAccount.getBalance();

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        // save the nilMessageTree Address in the json config for l2
        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);

        validateAddress(l2NetworkConfig.l2CommonConfig.owner, "l2CommonConfig.owner");
        validateAddress(l2NetworkConfig.l2CommonConfig.admin, "l2CommonConfig.admin");
        validateAddress(l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy, "l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy");

        const { tx: l2EnshrinedTokenBridgeImplDepTx, address: l2EnshrinedTokenBridgeImplAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: L2EnshrinedTokenBridgeJson.default.bytecode as `0x${string}`,
            abi: L2EnshrinedTokenBridgeJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await waitTillCompleted(deployerAccount.client, l2EnshrinedTokenBridgeImplDepTx.hash, {
            waitTillMainShard: true
        });

        if (!l2EnshrinedTokenBridgeImplDepTx || !l2EnshrinedTokenBridgeImplDepTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2EnshrinedTokenBridge Contract`);
        }

        if (!l2EnshrinedTokenBridgeImplAddress) {
            throw Error(`Invalid address output from deployContract call for L2EnshrinedTokenBridge Contract`);
        }

        console.log(`L2EnshrinedTokenBridge contract deployed at address: ${l2EnshrinedTokenBridgeImplAddress} and with transactionHash: ${l2EnshrinedTokenBridgeImplDepTx.hash}`);

        l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeImplementation = getCheckSummedAddress(l2EnshrinedTokenBridgeImplAddress);

        const initData = encodeFunctionData({
            abi: L2EnshrinedTokenBridgeJson.default.abi,
            functionName: "initialize",
            args: [l2NetworkConfig.l2CommonConfig.owner,
            l2NetworkConfig.l2CommonConfig.admin,
            l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy],
        });

        const { tx: proxyDeploymentTx, address: proxyAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string}`,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EnshrinedTokenBridgeImplAddress, deployerAccount.address, initData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });
        await waitTillCompleted(deployerAccount.client, proxyDeploymentTx.hash, {
            waitTillMainShard: true
        });

        l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy = getCheckSummedAddress(proxyAddress);

        const proxyContractInstance = getContract({
            client: deployerAccount.client,
            abi: TransparentUpgradeableProxy.default.abi,
            address: l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy as `0x${string}`,
        });

        const proxyAdminAddress = await proxyContractInstance.read.fetchAdmin([]);
        l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.proxyAdmin = getCheckSummedAddress(proxyAdminAddress as `0x${string}`);

        const l2EnshrinedTokenBridgeProxyInstance = getContract({
            client: deployerAccount.client,
            abi: L2EnshrinedTokenBridgeJson.default.abi as Abi,
            address: l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy as `0x${string}`
        });

        const l2EnshrinedTokenBridgeOwner = await l2EnshrinedTokenBridgeProxyInstance.read.owner([]);
        if (l2EnshrinedTokenBridgeOwner != l2NetworkConfig.l2CommonConfig.owner) {
            throw Error(`OwnerAddress in EnshrinedTokenBridgeContract: ${l2EnshrinedTokenBridgeOwner} is incorrect, correct owner as per config: ${l2NetworkConfig.l2CommonConfig.owner}`);
        }

        const implementationAddressFromContract = await l2EnshrinedTokenBridgeProxyInstance.read.getImplementation([]);
        if (implementationAddressFromContract != l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeImplementation) {
            throw Error(`L2EnshrinedBridgeImplementation in Proxy is incorrect`);
        }
        // Save the updated config
        saveNilNetworkConfig(networkName, l2NetworkConfig);
    });
