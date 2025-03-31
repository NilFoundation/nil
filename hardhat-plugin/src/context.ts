import type { Faucet, LocalECDSAKeySigner, PublicClient, WalletV1 } from "@nilfoundation/niljs";
import type { HardhatRuntimeEnvironment } from "hardhat/types";

export interface HandlerContext {
  hre: HardhatRuntimeEnvironment;
  client: PublicClient;
  wallet: WalletV1;
  faucet: Faucet;
  signer: LocalECDSAKeySigner;
  originalSend: (method: string, params: any[]) => Promise<any>;
  originalRequest: (args: { method: string; params: any[] }) => Promise<any>;
  isRequest: boolean; // Field indicating if the original call was a request or send
  feeCredit: bigint;
  directTxValue?: bigint;
  directTxFeeCredit?: bigint;
  debug: boolean;
}
