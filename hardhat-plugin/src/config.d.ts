import "hardhat/types/config";
import type { HardhatUserConfig } from "hardhat/types";

declare module "hardhat/types/config" {
  export interface NilHardhatUserConfig extends HardhatUserConfig {
    defaultShardId?: number;
  }

  export interface NetworkUserConfig {
    nil: boolean;
  }

  interface HardhatConfig extends NilHardhatUserConfig {} // Augmenting existing type
}
