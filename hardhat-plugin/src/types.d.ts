import "hardhat/types/config";

declare module "hardhat/types/config" {
  import type { HardhatUserConfig } from "hardhat/types";

  export interface NilHardhatUserConfig extends HardhatUserConfig {
    walletAddress?: string;
    defaultShardId?: number;
  }

  interface HardhatConfig extends NilHardhatUserConfig {} // Augmenting existing type
}
