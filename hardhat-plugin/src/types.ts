import type { CommonReadContractMethods, CommonWriteContractMethods, IAddress, PublicClient } from "@nilfoundation/niljs";
import type { SmartAccountInterface } from "@nilfoundation/niljs";

export type GetContractAtConfig = {
  publicClient?: PublicClient;
  smartAccount?: SmartAccountInterface;
}

declare module 'hardhat/types/runtime' {
    interface Network {
        zksync: boolean;
    }

    interface HardhatRuntimeEnvironment {
        nil: {
            provider: PublicClient;
            getPublicClient: () => PublicClient;
            getSmartAccount: () => Promise<SmartAccountInterface>;
            getContractAt: (
            contractName: string,
            address: IAddress,
            config: GetContractAtConfig
            ) => Promise<{
            read: CommonReadContractMethods;
            write: CommonWriteContractMethods;
            }>;
        };
    }
}