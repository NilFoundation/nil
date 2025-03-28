import { processImports } from "./processImports";

// biome-ignore lint/suspicious/noExplicitAny: <explanation>
export const createCompileInput = async (
  contractBody: string,
  options: any = {},
): Promise<object> => {
  const sources: Record<string, { content: string }> = {
    Compiled_Contracts: {
      content: contractBody,
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
