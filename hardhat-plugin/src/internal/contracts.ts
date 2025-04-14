import { type IAddress, getContract } from "@nilfoundation/niljs";
import type { HardhatRuntimeEnvironment } from "hardhat/types";
import type { GetContractAtConfig } from "../types";

export const getContractAt = async (
  { artifacts, network, nil }: HardhatRuntimeEnvironment,
  contractName: string,
  address: IAddress,
  config?: GetContractAtConfig,
) => {
  const [publicClient, smartAccount, contractArtifact] = await Promise.all([
    config?.publicClient ?? nil.getPublicClient(),
    config?.smartAccount ?? nil.getSmartAccount(),
    artifacts.readArtifact(contractName),
  ]);

  if (config?.signer) {
    return getContract({
      abi: contractArtifact.abi,
      address,
      client: publicClient,
      smartAccount: smartAccount,
      externalInterface: {
        signer: config.signer,
        methods: config?.externalMethods || [],
      }
    });
  }

  return getContract({
    abi: contractArtifact.abi,
    address,
    client: publicClient,
    smartAccount: smartAccount
  });
};
