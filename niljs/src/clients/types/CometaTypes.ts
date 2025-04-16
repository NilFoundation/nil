/**
 * @fileoverview Types for the Cometa client.
 */

import type { Hex } from "../../types/Hex.js";

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
  initCode: string;
  code: string;
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

/**
 * Transaction data.
 */
type TransactionData = {
  address: Hex;
  funcId: Hex;
};

export type { ContractData, Location, TransactionData };
