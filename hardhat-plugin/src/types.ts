import type {
  CommonReadContractMethods,
  CommonWriteContractMethods,
  IAddress,
  ISigner,
  PublicClient,
  SmartAccountInterface,
} from "@nilfoundation/niljs";

export type GetContractAtConfig = {
  publicClient?: PublicClient;
  smartAccount?: SmartAccountInterface;
  signer?: ISigner;
  externalMethods?: string[];
};

export type DeployContractConfig = {
  shardId?: number;
} & GetContractAtConfig;

export type CreateSmartAccountConfig = {
  topUp?: boolean;
};

export type GetContractAtConfigWithSigner = GetContractAtConfig & {
  signer: ISigner;
};

export declare function getContractAt(
  contractName: string,
  address: IAddress,
  config?: GetContractAtConfig,
): Promise<{
  read: CommonReadContractMethods;
  write: CommonWriteContractMethods;
}>;

export declare function deployContract(
  contractName: string,
  args: unknown[],
  config?: DeployContractConfig,
): Promise<{
  address: IAddress;
  read: CommonReadContractMethods;
  write: CommonWriteContractMethods;
}>;

export declare function createSmartAccount(
  config: CreateSmartAccountConfig,
): Promise<SmartAccountInterface>;

export type NilHelper = {
  provider: PublicClient;
  getPublicClient: () => PublicClient;
  getSmartAccount: () => Promise<SmartAccountInterface>;
  getContractAt: typeof getContractAt;
  deployContract: typeof deployContract;
  createSmartAccount: typeof createSmartAccount;
};

declare module "hardhat/types/runtime" {

  interface HardhatRuntimeEnvironment {
    nil: NilHelper;
  }
}
