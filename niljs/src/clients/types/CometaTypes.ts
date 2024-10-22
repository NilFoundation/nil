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
	// biome-ignore lint/suspicious/noExplicitAny: <explanation>
	abi: any[];
	sourceCode: Record<string, string>;
	sourceMap: string;
	metadata: string;
	deployCode: Uint8Array;
	code: Uint8Array;
	sourceFilesList: string[];
	// biome-ignore lint/suspicious/noExplicitAny: <explanation>
	compilerOutput: any;
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
