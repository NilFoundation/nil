import { extendEnvironment } from "hardhat/config";
import "./tasks/wallet";
import {
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  SmartAccountV1,
} from "@nilfoundation/niljs";
import "./tasks/subtasks";
import { generateRandomPrivateKey } from "@nilfoundation/niljs";
import { getContractAt, deployContract } from "./internal/contracts";

extendEnvironment((hre) => {
  if ("nil" in hre.network.config && hre.network.config.nil) {
    if (!("url" in hre.network.config)) {
      throw new Error("Nil network config is missing url");
    }
    const url = hre.network.config.url;
    const nilProvider = new HttpTransport({
      endpoint: url,
    });
    const publicClient = new PublicClient({
      transport: nilProvider,
    });
    const defaultSharId = hre.config.defaultShardId ?? 1;

    const pk = generateRandomPrivateKey();
    const signer = new LocalECDSAKeySigner({
      privateKey: pk,
    });

    hre.nil = {
      provider: publicClient,
      getPublicClient: () => {
        return publicClient;
      },
      getSmartAccount: async () => {
        const smartAccount = new SmartAccountV1({
          client: publicClient,
          signer: signer,
          pubkey: signer.getPublicKey(),
          shardId: defaultSharId,
          salt: 1n,
        });

        // try {
        //   await smartAccount.selfDeploy(true)
        // } catch (e) {
        //   if (typeof e === 'object' && e !== null && 'message' in e && typeof e.message ==='string' && e.message.includes("already deployed")) {
        //     return smartAccount;
        //   }
        //   throw new Error(`Failed to deploy smart account: ${e}`);
        // }
        return smartAccount;
      },
      getContractAt: async (contractName, address, config) => {
        return getContractAt(hre, contractName, address, config);
      },
      deployContract: async (contractName, args, config) => {
        return deployContract(hre, contractName, args, config);
      }
    };
  }
});
export type * from "./types";
export type * from "./config";
