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
import { fetchConfigIni } from "./config/config";
import { getContractAt } from "./internal/contracts";

extendEnvironment((hre) => {
  (hre as any).smartAccount = async (): Promise<SmartAccountV1> => {
    const config = fetchConfigIni();
    const signer = new LocalECDSAKeySigner({
      // @ts-ignore
      privateKey: config.privateKey,
    });
    const pubkey = signer.getPublicKey();

    const publicClient = new PublicClient({
      transport: new HttpTransport({
        endpoint: config.rpcEndpoint,
      }),
      shardId: 1,
    });

    return new SmartAccountV1({
      // @ts-ignore
      address: config.address,
      client: publicClient,
      signer: signer,
      shardId: 1,
    });
  };

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
    const defaultSharId = 1;

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
    };
  }
});
export type * from "./types";
export type * from "./config";
