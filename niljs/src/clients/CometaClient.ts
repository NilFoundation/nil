import type { Hex } from "../types/Hex.js";
import { BaseClient } from "./BaseClient.js";
import type { ContractData, Location } from "./types/CometaTypes.js";
import type { CometaClientConfig } from "./types/Configs.js";

/**
 * CometaClient is a client that interacts with the Cometa service.
 * Cometa service is used to store contract metadata: source code, ABI, etc.
 * @class CometaClient
 * @extends BaseClient
 * @example
 * import { CometaClient } from '@nilfoundation/niljs';
 *
 * const service = new CometaClient({
 *   transport: new HttpTransport({
 *     endpoint: COMETA_ENDPOINT,
 *   }),
 * });
 */
class CometaClient extends BaseClient {
  // biome-ignore lint/complexity/noUselessConstructor: may be useful in the future
  constructor(config: CometaClientConfig) {
    super(config);
  }

  /**
   * Returns the contract metadata.
   * @param address - Address of the contract.
   * @returns The contract metadata.
   */
  public async getContract(address: Hex) {
    return await this.request<ContractData>({
      method: "cometa_getContract",
      params: [address],
    });
  }

  /**
   * Returns the contract metadata.
   * @param address - Address of the contract.
   * @param pc - Program counter.
   * @returns The contract metadata.
   */
  public async getLocation(address: Hex, pc: number) {
    return await this.request<Location>({
      method: "cometa_getLocation",
      params: [address, pc],
    });
  }

  /**
   * Compiles the contract.
   * @param inputJson - The JSON input.
   * @returns The contract metadata.
   */
  public async compileContract(inputJson: string) {
    return await this.request<ContractData>({
      method: "cometa_compileContract",
      params: [inputJson],
    });
  }

  /**
   * Register the contract by compilation result.
   * @param contractData - The contract data.
   * @param address - Address of the contract.
   */
  public async registerContractData(contractData: ContractData, address: Hex) {
    return await this.request({
      method: "cometa_registerContractData",
      params: [contractData, address],
    });
  }

  /**
   * Register the contract.
   * @param inputJson - The JSON input for compiler.
   * @param address - Address of the contract.
   */
  public async registerContract(inputJson: string, address: Hex) {
    return await this.request({
      method: "cometa_registerContract",
      params: [inputJson, address],
    });
  }
}

export { CometaClient };
