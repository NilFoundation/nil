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
    ProcessedReceipt,
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { getCheckSummedAddress } from "../scripts/utils/validate-config";
import { decodeFunctionResult, encodeFunctionData } from "viem";

// npx hardhat set-eth-bridge-vault-dependencies --networkname local
task("set-eth-bridge-vault-dependencies", "Set dependencies of L2ETHBridgeVault contract")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const L2ETHBridgeVaultJson = await import("../artifacts/contracts/bridge/l2/L2ETHBridgeVault.sol/L2ETHBridgeVault.json");
        if (!L2ETHBridgeVaultJson || !L2ETHBridgeVaultJson.default || !L2ETHBridgeVaultJson.default.abi || !L2ETHBridgeVaultJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridgeVaultJson ABI`);
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

        const setL2ETHBridgeData = encodeFunctionData({
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            functionName: "setL2ETHBridge",
            args: [l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy],
        });

        const setL2ETHBridgeResponse = await deployerAccount.sendTransaction({
            to: l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy as `0x${string}`,
            data: setL2ETHBridgeData,
            feeCredit: convertEthToWei(0.001),
        });

        const setL2ETHBridgeResponseTxnReceipt: ProcessedReceipt[] = await setL2ETHBridgeResponse.wait();

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!setL2ETHBridgeResponseTxnReceipt[0].success) {
            throw Error(`Failed to set L2ETHBridge: ${l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy} 
            on the L2ETHBridgeVault contract: ${l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy}`);
        }

        // verify if the L2ETHBridge is set
        const l2ETHBridgeVaultProxyInstance = getContract({
            client: deployerAccount.client,
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            address: l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy as `0x${string}`
        });

        const l2ETHBridgeFromVaultContract = await l2ETHBridgeVaultProxyInstance.read.l2ETHBridge([]);
        if (!l2ETHBridgeFromVaultContract || l2ETHBridgeFromVaultContract != l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy) {
            throw Error(`Invalid L2ETHBridge: ${l2ETHBridgeFromVaultContract} was set in L2ETHBridgeVault. expected L2ETHBridge from Vault: ${l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy}`);
        }
    });
