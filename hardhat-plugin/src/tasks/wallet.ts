import {
  FaucetClient,
  HttpTransport,
  type ILocalKeySignerConfig,
  LocalECDSAKeySigner,
  PublicClient,
} from "@nilfoundation/niljs";
import { scope } from "hardhat/config";
import type { HardhatRuntimeEnvironment } from "hardhat/types";
import { fetchConfigIni } from "../config/config";
import { deployWallet } from "../core/wallet";

const walletTask = scope("wallet", "Wallet tasks");

walletTask
  .task("deploy", "Deploy a new wallet if the current one doesn't exist")
  .setAction(async (taskArgs, hre: HardhatRuntimeEnvironment) => {
    const configIni = fetchConfigIni();
    const signer = new LocalECDSAKeySigner(<ILocalKeySignerConfig>{
      privateKey: configIni.privateKey,
    });
    const client = new PublicClient({
      transport: new HttpTransport({
        endpoint: configIni.rpcEndpoint,
      }),
      shardId: 1,
    });
    const faucetClient = new FaucetClient({
      transport: new HttpTransport({ endpoint: configIni.rpcEndpoint }),
    });

    const smartAccountV1 = await deployWallet(
      signer,
      <`0x${string}`>configIni.address,
      client,
      faucetClient,
    );

    console.log("Smart account deployed at", smartAccountV1.address);
  });

walletTask
  .task("info", "Print info about current wallet")
  .setAction(async (taskArgs, hre: HardhatRuntimeEnvironment) => {
    const nilConfigIni = fetchConfigIni();

    console.log("Current wallet:");
    console.log(`  address: ${nilConfigIni.address}`);
    console.log(`  privateKey: ${nilConfigIni.privateKey}`);
  });
