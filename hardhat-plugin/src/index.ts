import { extendEnvironment } from "hardhat/config";
import type { HardhatUserConfig } from "hardhat/types";
import { unifiedInterceptor } from "./interceptors";
import { setupWalletAndClient } from "./setup";

extendEnvironment((hre) => {
  const originalRequest = hre.network.provider.request.bind(
    hre.network.provider,
  );
  const originalSend = hre.network.provider.send.bind(hre.network.provider);
  const contextPromise = setupWalletAndClient(
    hre,
    originalRequest,
    originalSend,
  );

  hre.network.provider.send = async (method, params) => {
    const context = await contextPromise;
    context.isRequest = false;
    return unifiedInterceptor(method, params || [], context);
  };

  hre.network.provider.request = async (args) => {
    const context = await contextPromise;
    context.isRequest = true;
    const safeParams = Array.isArray(args.params)
      ? args.params
      : args.params
        ? [args.params]
        : [];
    return unifiedInterceptor(args.method, safeParams, context);
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
