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

// npx hardhat authorise-l2-bridges-in-messenger --networkname local
task("authorise-l2-bridges-in-messenger", "Authorises L2Bridge contracts to send messages via L2BridgeMessenger from Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const L2BridgeMessengerJson = await import("../artifacts/contracts/bridge/l2/L2BridgeMessenger.sol/L2BridgeMessenger.json");
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

        const authoriseBridgesData = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi as Abi,
            functionName: "authoriseBridges",
            args: [[l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy,
            l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy]
            ],
        });

        const authoriseL2BridgesResponse = await deployerAccount.sendTransaction({
            to: l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy as `0x${string}`,
            data: authoriseBridgesData,
            feeCredit: convertEthToWei(0.001),
        });

        const authoriseL2BridgesTxnReceipts: ProcessedReceipt[] = await authoriseL2BridgesResponse.wait();

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!authoriseL2BridgesTxnReceipts[0].success) {
            throw Error(`Failed to authorise Bridges: ${[l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy,
            l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy]} 
            on the L2BridgeMessenger contract: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy}`);
        }

        // verify if the bridges are really authorised
        const l2BridgeMessengerProxyInstance = getContract({
            client: deployerAccount.client,
            abi: L2BridgeMessengerJson.default.abi as Abi,
            address: l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy as `0x${string}`
        });

        const isL2EnshrinedTokenBridgeAuthorised = await l2BridgeMessengerProxyInstance.read.isAuthorisedBridge([l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy]);
        if (!isL2EnshrinedTokenBridgeAuthorised) {
            throw Error(`L2EnshrinedTokenBridge: ${l2NetworkConfig.l2EnshrinedTokenBridgeConfig.l2EnshrinedTokenBridgeContracts.l2EnshrinedTokenBridgeProxy} is not authorised on L2BridgeMessenger: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy}`);
        }

        const isL2ETHBridgeAuthorised = await l2BridgeMessengerProxyInstance.read.isAuthorisedBridge([l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy]);
        if (!isL2ETHBridgeAuthorised) {
            throw Error(`L2ETHBridge: ${l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy} is not authorised on L2BridgeMessenger: ${l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy}`);
        }
    });
