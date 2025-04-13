import { processImports } from "./processImports";

import FaucetSol from "@nilfoundation/smart-contracts/contracts/Faucet.sol?raw";
import NilSol from "@nilfoundation/smart-contracts/contracts/Nil.sol?raw";
import NilTokBaseSol from "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol?raw";
import SmartAccountSol from "@nilfoundation/smart-contracts/contracts/SmartAccount.sol?raw";

// biome-ignore lint/suspicious/noExplicitAny: <explanation>
export const createCompileInput = async (
  contractBody: string,
  options: any = {},
): Promise<object> => {
  const sources: Record<string, { content: string }> = {
    Compiled_Contracts: {
      content: contractBody,
    },
    "Faucet.sol": { content: FaucetSol },
    "@nilfoundation/smart-contracts/contracts/Faucet.sol": {
      content: FaucetSol,
    },
    "Nil.sol": { content: NilSol },
    "@nilfoundation/smart-contracts/contracts/Nil.sol": { content: NilSol },
    "NilTokenBase.sol": { content: NilTokBaseSol },
    "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol": {
      content: NilTokBaseSol,
    },
    "SmartAccount.sol": { content: SmartAccountSol },
    "@nilfoundation/smart-contracts/contracts/SmartAccount.sol": {
      content: SmartAccountSol,
    },
  };

  await processImports(contractBody, "", sources);

  const CompileInput = {
    language: "Solidity",
    sources,
    settings: {
      metadata: {
        appendCBOR: false,
        bytecodeHash: "none",
      },
      debug: {
        debugInfo: ["location"],
      },
      outputSelection: {
        "*": {
          "*": ["*"],
        },
      },
      evmVersion: "cancun",
      optimizer: {
        enabled: false,
        runs: 200,
      },
      ...options,
    },
  };
  return CompileInput;
};
