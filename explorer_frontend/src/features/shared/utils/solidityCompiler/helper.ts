// TODO: get rid of hardcoded imports
import FaucetSol from "@nilfoundation/smart-contracts/contracts/Faucet.sol";
import NilSol from "@nilfoundation/smart-contracts/contracts/Nil.sol";
import NilTokBaseSol from "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import SmartAccountSol from "@nilfoundation/smart-contracts/contracts/SmartAccount.sol";

// biome-ignore lint/suspicious/noExplicitAny: <explanation>
export const createCompileInput = async (contractBody: string, options: any = {}): Promise<object> => {
  const sources: Record<string, { content: string }> = {
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
  };

  // Extract import statements from the contract body
  const importRegex = /import\s+["']([^"']+)["']/g;
  const imports = [...contractBody.matchAll(importRegex)].map(match => match[1]);

  // Fetch content for each import from unpkg.com
  for (const importPath of imports) {
    console.log("importPath", importPath);
    try {
      const response = await fetch(`https://unpkg.com/${importPath}`);
      if (response.ok) {
        const content = await response.text();
        sources[importPath] = { content };
      }
    } catch (error) {
      console.error(`Failed to fetch ${importPath}:`, error);
    }
  }

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
