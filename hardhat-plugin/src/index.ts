import { extendEnvironment } from "hardhat/config";
import "./tasks/cometa";
import "./tasks/wallet";
import {
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  SmartAccountV1,
} from "@nilfoundation/niljs";
import { fetchConfigIni } from "./config/config";

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
});

export * from "./config/config";
