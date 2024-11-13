import { readFileSync } from "node:fs";
import {
  CometaService,
  type Hex,
  HttpTransport,
  getShardIdFromAddress,
} from "@nilfoundation/niljs";
import type { Artifact, HardhatRuntimeEnvironment } from "hardhat/types";
import { getValue } from "./config";

type CompilerTask = {
  ContractName: string;
  CompilerVersion: string;
  BasePath: string;
  Sources: Record<string, Source>;
  Settings: Settings;
};

type Source = {
  Keccak256: string;
  Urls: string[];
  Content: string;
};

type Settings = {
  Optimizer?: Optimizer;
  EvmVersion?: string;
  AppendCBOR?: boolean;
  BytecodeHash?: string;
};

type Optimizer = {
  Enabled?: boolean;
  Runs?: number;
  Details?: any;
};

export const createCompileTask = async (
  artifact: Artifact,
  hre: HardhatRuntimeEnvironment,
): Promise<CompilerTask> => {
  const fullName = `${artifact.sourceName}:${artifact.contractName}`;
  const buildInfo = await hre.artifacts.getBuildInfoSync(fullName);
  if (buildInfo === undefined) {
    throw Error(`Cannot get BuildInfo for ${fullName}`);
  }

  const sources: Record<string, any> = {};
  for (const src in buildInfo.output.sources) {
    let fileName = src;
    if (src[0] === "@") {
      // TODO: find a better way to find the path
      fileName = `node_modules/${src}`;
    }
    const data = readFileSync(fileName);
    sources[src] = {
      content: data.toString(),
    };
  }

  if (buildInfo.input.settings.evmVersion === undefined) {
    throw Error("EvmVersion is not defined");
  }

  return {
    ContractName: `${artifact.sourceName}:${artifact.contractName}`,
    CompilerVersion: buildInfo.solcVersion,
    BasePath: "./",
    Sources: sources,
    Settings: {
      Optimizer: {
        Enabled: buildInfo.input.settings.optimizer.enabled,
        Runs: buildInfo.input.settings.optimizer.runs,
        Details: {},
      },
      EvmVersion: buildInfo.input.settings.evmVersion,
      // TODO: We need to get this from the compiler settings. But looks like hardhat doesn't provide this information.
      AppendCBOR: false,
      BytecodeHash: "none",
    },
  };
};

export async function registerContract(
  contract: string,
  address: Hex,
  hre: HardhatRuntimeEnvironment,
) {
  const artifact = await hre.artifacts.readArtifact(contract);
  const endpoint = getValue("cometa_endpoint");
  if (!endpoint) {
    throw Error("Cometa endpoint is not set");
  }
  const cometa = new CometaService({
    transport: new HttpTransport({
      endpoint: endpoint,
    }),
    shardId: getShardIdFromAddress(address),
  });
  const task = await createCompileTask(artifact, hre);
  await cometa.registerContract(JSON.stringify(task), address);
}
