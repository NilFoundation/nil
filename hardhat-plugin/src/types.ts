import type { CommonReadContractMethods, CommonWriteContractMethods, IAddress, PublicClient } from "@nilfoundation/niljs";
import type { SmartAccountInterface } from "@nilfoundation/niljs";

export type GetContractAtConfig = {
  publicClient?: PublicClient;
  smartAccount?: SmartAccountInterface;
}

export declare function getContractAt(
  contractName: string,
  address: IAddress,
  config?: GetContractAtConfig
): Promise<{
  read: CommonReadContractMethods;
  write: CommonWriteContractMethods;
}>;

export type NilHelper = {
    provider: PublicClient;
    getPublicClient: () => PublicClient;
    getSmartAccount: () => Promise<SmartAccountInterface>;
    getContractAt: typeof getContractAt;
};

declare module 'hardhat/types/runtime' {
    interface Network {
        zksync: boolean;
    }

    interface HardhatRuntimeEnvironment {
        nil: NilHelper;
    }
}