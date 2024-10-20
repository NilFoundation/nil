// TODO: get rid of hardcoded imports
import FaucetSol from "@nilfoundation/smart-contracts/contracts/Faucet.sol";
import NilCurBaseSol from "@nilfoundation/smart-contracts/contracts/NilCurrencyBase.sol";
import NilSol from "@nilfoundation/smart-contracts/contracts/Nil.sol";
import WalletSol from "@nilfoundation/smart-contracts/contracts/Wallet.sol";

// biome-ignore lint/suspicious/noExplicitAny: it could be any options here
export const createCompileInput = (contractBody: string, options: any = {}): string => {
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
      "NilCurrencyBase.sol": {
        content: NilCurBaseSol,
      },
      "@nilfoundation/smart-contracts/contracts/NilCurrencyBase.sol": {
        content: NilCurBaseSol,
      },
      "Nil.sol": {
        content: NilSol,
      },
      "@nilfoundation/smart-contracts/contracts/Nil.sol": {
        content: NilSol,
      },
      "Wallet.sol": {
        content: WalletSol,
      },
      "@nilfoundation/smart-contracts/contracts/Wallet.sol": {
        content: WalletSol,
      },
    },
    settings: {
      metadata: { appendCBOR: false },
      outputSelection: {
        "*": {
          "*": ["*"],
        },
      },
      ...options,
    },
  };
  return JSON.stringify(CompileInput);
};
