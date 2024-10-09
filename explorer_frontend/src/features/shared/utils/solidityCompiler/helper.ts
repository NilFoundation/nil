import NilSol from "@nilfoundation/smart-contracts/contracts/Nil.sol";

// biome-ignore lint/suspicious/noExplicitAny: it could be any options here
export const createCompileInput = (contractBody: string, options: any = {}): string => {
  const CompileInput = {
    language: "Solidity",
    sources: {
      Compiled_Contracts: {
        content: contractBody,
      },
      "Nil.sol": {
        content: NilSol,
      },
      "@nilfoundation/smart-contracts/contracts/Nil.sol": {
        content: NilSol,
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
