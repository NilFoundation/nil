import "hardhat/types/config";
import type { HardhatUserConfig } from "hardhat/types";

declare module "hardhat/types/config" {
  export interface NilHardhatUserConfig extends HardhatUserConfig {
    walletAddress?: string;
    defaultShardId?: number;
  }

  interface HardhatConfig extends NilHardhatUserConfig {} // Augmenting existing type

  interface HttpNetworkUserConfig {
    nil?: boolean;
  }
}
