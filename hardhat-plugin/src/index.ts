import type { Hex } from "@nilfoundation/niljs";
import { extendEnvironment } from "hardhat/config";
import type { HardhatUserConfig } from "hardhat/types";
import { registerContract } from "./cometa";
import { unifiedInterceptor } from "./interceptors";
import { setupWalletAndClient } from "./setup";
import "./tasks/cometa";
import "./tasks/wallet";
import type { HandlerContext } from "./context";

extendEnvironment((hre) => {
  const originalRequest = hre.network.provider.request.bind(hre.network.provider);
  const originalSend = hre.network.provider.send.bind(hre.network.provider);
  let context: HandlerContext | null = null;

  hre.network.provider.send = async (method, params) => {
    context ||= await setupWalletAndClient(hre, originalRequest, originalSend);
    context.isRequest = false;
    return unifiedInterceptor(method, params || [], context);
  };

  hre.network.provider.request = async (args) => {
    context ||= await setupWalletAndClient(hre, originalRequest, originalSend);
    context.isRequest = true;
    const safeParams = Array.isArray(args.params) ? args.params : args.params ? [args.params] : [];
    return unifiedInterceptor(args.method, safeParams, context);
  };

  (hre as any).registerContract = async (contract: string, address: Hex) => {
    return registerContract(contract, address, hre);
  };
});

export interface NilHardhatUserConfig extends HardhatUserConfig {
  walletAddress?: string;
  feeCredit?: number;
  shardId?: number;
  directTxGasLimit?: number;
  directTxValue?: number;
  debug?: boolean;
}

export * from "./config";
