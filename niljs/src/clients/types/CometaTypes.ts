/**
 * @fileoverview Types for the Cometa client.
 */

/**
 * Contract data.
 * @typedef {Object} ContractData
 */
type ContractData = {
  name: string;
  description: string;
  abi: string;
  sourceCode: Record<string, string>;
  sourceMap: string;
  metadata: string;
  initCode: Uint8Array;
  code: Uint8Array;
  sourceFilesList: string[];
  methodIdentifiers: Record<string, string>;
};

/**
 * Location.
 * @typedef {Object} Location
 */
type Location = {
  fileName: string;
  position: number;
  length: number;
};

export type { ContractData, Location };
