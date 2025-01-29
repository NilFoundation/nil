// TODO: get rid of hardcoded imports
import FaucetSol from "@nilfoundation/smart-contracts/contracts/Faucet.sol";
import NilSol from "@nilfoundation/smart-contracts/contracts/Nil.sol";
import NilTokBaseSol from "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import SmartAccountSol from "@nilfoundation/smart-contracts/contracts/SmartAccount.sol";

// biome-ignore lint/suspicious/noExplicitAny: <explanation>
export const createCompileInput = (contractBody: string, options: any = {}): object => {
  const CompileInput = {
    language: "Solidity",
    sources: {
      Compiled_Contracts: {
        content: contractBody,
      },
      "Faucet.sol": {
        content: FaucetSol,
      },
      "@nilfoundation/smart-contracts/contracts/Faucet.sol": {
        content: FaucetSol,
      },
      "NilTokenBase.sol": {
        content: NilTokBaseSol,
      },
      "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol": {
        content: NilTokBaseSol,
      },
      "Nil.sol": {
        content: NilSol,
      },
      "@nilfoundation/smart-contracts/contracts/Nil.sol": {
        content: NilSol,
      },
      "SmartAccount.sol": {
        content: SmartAccountSol,
      },
      "@nilfoundation/smart-contracts/contracts/SmartAccount.sol": {
        content: SmartAccountSol,
      },
    },
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
