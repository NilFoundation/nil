import "hardhat/types/config";

declare module "hardhat/types/config" {

  interface HardhatConfig {
    defaultShardId?: number;
  }

  interface HttpNetworkUserConfig {
    nil?: boolean;
  }
}
