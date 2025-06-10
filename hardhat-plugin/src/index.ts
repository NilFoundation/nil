import { extendEnvironment } from "hardhat/config";
import "./tasks/wallet";
import {
  FaucetClient,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  SmartAccountV1,
} from "@nilfoundation/niljs";
import "./tasks/subtasks";
import { createSmartAccount } from "./core/wallet";
import { deployContract, getContractAt } from "./internal/contracts";

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
    const faucetClient = new FaucetClient({
      transport: nilProvider,
    });

    const pk = <`0x${string}`>`0x${process.env.PRIVATE_KEY}`;
    const signer = new LocalECDSAKeySigner({
      privateKey: pk,
    });

    hre.nil = {
      provider: publicClient,
      getFaucetClient: () => {
        return faucetClient;
      },
      getPublicClient: () => {
        return publicClient;
      },
      getSmartAccount: async () => {
        if (!process.env.SMART_ACCOUNT_ADDR) {
          throw new Error(
            "SMART_ACCOUNT_ADDR is missing. Run 'npx run create-smart-account' to create a new one",
          );
        }
        const address = <`0x${string}`>process.env.SMART_ACCOUNT_ADDR;
        return new SmartAccountV1({
          client: publicClient,
          pubkey: signer.getPublicKey(),
          signer: signer,
          address: address,
        });
      },
      getContractAt: async (contractName, address, config) => {
        return getContractAt(hre, contractName, address, config);
      },
      deployContract: async (contractName, args, config) => {
        return deployContract(hre, contractName, args, config);
      },
      createSmartAccount: async (config) => {
        return createSmartAccount(hre, config);
      },
    };
  }
});

export type * from "./types";
export { topUpSmartAccount } from "./core/wallet";
