import type {
  CommonReadContractMethods,
  CommonWriteContractMethods,
  FaucetClient,
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
  value?: bigint;
  feeCredit?: bigint;
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
  getFaucetClient: () => FaucetClient;
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
