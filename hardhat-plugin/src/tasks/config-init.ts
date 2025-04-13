import { LocalECDSAKeySigner, generateRandomPrivateKey } from "@nilfoundation/niljs";
import { SmartAccountV1 } from "@nilfoundation/niljs";
import { scope } from "hardhat/config";
import { types } from "hardhat/internal/core/config/config-env";
import type { HardhatRuntimeEnvironment } from "hardhat/types";
import { type NilConfigIni, saveConfig } from "../config/config";

const walletTask = scope("wallet", "Wallet tasks");

walletTask
  .task("config-init", "Init a new config.ini file")
  .addOptionalParam(
    "force",
    "Rewrite the config.ini file even if it already exists",
    false,
    types.boolean,
  )
  .addOptionalParam("shardId", "Shard ID to use for the wallet", 1, types.int)
  .setAction(async (taskArgs, hre: HardhatRuntimeEnvironment) => {
    const random = Math.round(Math.random() * 1000000);
    const privateKey = generateRandomPrivateKey();
    const signer = new LocalECDSAKeySigner({
      privateKey: privateKey,
    });
    const pubkey = signer.getPublicKey();

    const accountAddress = SmartAccountV1.calculateSmartAccountAddress({
      pubKey: pubkey,
      shardId: taskArgs.shardId,
      salt: BigInt(random),
    });
    const config: NilConfigIni = {
      rpcEndpoint: "http://127.0.0.1:8529",
      cometaEndpoint: "http://127.0.0.1:8529",
      privateKey,
      address: accountAddress,
    };

    const configPath = saveConfig(config, taskArgs.force);

    console.log(`Config file created at ${configPath}`);
  });
