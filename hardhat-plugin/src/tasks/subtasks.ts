import type { Artifact, Artifacts } from "hardhat/types";
import type { ArtifactsEmittedPerFile } from "hardhat/types/builtin-tasks";

import { mkdir, readFile, rm, writeFile, } from "node:fs/promises";
import { dirname, join, relative } from "node:path";

import { parse, visit } from "@solidity-parser/parser";
import {
  TASK_COMPILE_REMOVE_OBSOLETE_ARTIFACTS,
  TASK_COMPILE_SOLIDITY,
  TASK_COMPILE_SOLIDITY_EMIT_ARTIFACTS,
} from "hardhat/builtin-tasks/task-names";
import { subtask } from "hardhat/config";
import { getAllFilesMatching } from "hardhat/internal/util/fs-utils";
import { getFullyQualifiedName, parseFullyQualifiedName } from "hardhat/utils/contract-names";
import { replaceBackslashes } from "hardhat/utils/source-names";
import type { VariableDeclaration } from "@solidity-parser/parser/dist/src/ast-types";

interface EmittedArtifacts {
  artifactsEmittedPerFile: ArtifactsEmittedPerFile;
}

/**
 * Override task that generates an `artifacts.nil.d.ts` file with `never`
 * types for duplicate contract names. This file is used in conjunction with
 * the `artifacts.nil.d.ts` file inside each contract directory to type
 * `hre.artifacts`.
 */
subtask(TASK_COMPILE_SOLIDITY).setAction(async (_, { config, artifacts }, runSuper) => {
  const superRes = await runSuper();

  const duplicateContractNames = await findDuplicateContractNames(artifacts);

  const duplicateArtifactsDTs = generateDuplicateArtifactsDefinition(duplicateContractNames);

  try {
    await writeFile(join(config.paths.artifacts, "artifacts-nil.d.ts"), duplicateArtifactsDTs);
  } catch (error) {
    console.error("Error writing artifacts definition:", error);
  }

  return superRes;
});

/**
 * Override task to emit TypeScript and definition files for each contract.
 * Generates a `.d.ts` file per contract, and a `artifacts-nil.d.ts` per solidity
 * file, which is used in conjunction to the root `artifacts-nil.d.ts`
 * to type `hre.artifacts`.
 */
subtask(TASK_COMPILE_SOLIDITY_EMIT_ARTIFACTS).setAction(
  async (_, { artifacts, config }, runSuper): Promise<EmittedArtifacts> => {
    const { artifactsEmittedPerFile }: EmittedArtifacts = await runSuper();
    const duplicateContractNames = await findDuplicateContractNames(artifacts);

    await Promise.all(
      artifactsEmittedPerFile.map(async ({ file, artifactsEmitted }) => {

        const functionsUsingModifier: Array<{name: string, parameters: VariableDeclaration[]}> = [];
        const source = await readFile(file.absolutePath, "utf8");
        const ast = parse(source, { range: true });
        visit(ast, {
          FunctionDefinition(node) {
            if (node.modifiers && node.modifiers.length > 0) {
              // проверяем, есть ли среди модификаторов `onlyExternal`
              const hasOnlyExternal = node.modifiers.some(
                (modifier) => modifier.name === "onlyExternal",
              );
              if (hasOnlyExternal) {
                // Сохраняем информацию (имя, сигнатуру, позицию в файле)
                const functionName = node.name || "<constructor or fallback>";
                const functionSignature = node.parameters;
                functionsUsingModifier.push(
                 {name: functionName, parameters: functionSignature},
                );
              }
            }
          },
        });

        const srcDir = join(config.paths.artifacts, file.sourceName);
        await mkdir(srcDir, {
          recursive: true,
        });
        
        const contractTypeData = await Promise.all(
          artifactsEmitted.map(async (contractName) => {
            const fqn = getFullyQualifiedName(file.sourceName, contractName);
            const artifact = await artifacts.readArtifact(fqn);
            const isDuplicate = duplicateContractNames.has(contractName);
            const declaration = generateContractDeclaration(artifact, isDuplicate);

            const typeName = `${contractName}$NilType`;

            return { contractName, fqn, typeName, declaration };
          }),
        );

        const fp: Array<Promise<void>> = [];
        for (const { contractName, declaration } of contractTypeData) {
          const contractJSON = await readFile(join(srcDir, `${contractName}.json`), "utf8");
          const contractData = JSON.parse(contractJSON);
          const abi = contractData.abi;
          let hasSomeUpdates = false;
          for (const item of abi) {
            if (item.type === "function") {
              const hasOnlyExtenralModifier = functionsUsingModifier.some(
                (func) => {
                  if (func.name === item.name && item.inputs.length === func.parameters.length) {
                    for (let i=0; i<func.parameters.length; i++) {
                      if (func.parameters[i].name !== item.inputs[i].name) {
                        return false;
                      }
                    }
                    return true;
                  }
                  return false;
                },
              );
              if (hasOnlyExtenralModifier) {
                item.onlyExternal = true;
                hasSomeUpdates = true;
              }
            }
          }

          if (hasSomeUpdates) {
            await writeFile(join(srcDir, `${contractName}.json`), JSON.stringify(contractData, null, 2));
          }

          fp.push(writeFile(join(srcDir, `${contractName}.d.ts`), declaration));
        }

        const dTs = generateArtifactsDefinition(contractTypeData);
        fp.push(writeFile(join(srcDir, "artifacts-nil.d.ts"), dTs));

        try {
          await Promise.all(fp);
        } catch (error) {
          console.error("Error writing artifacts definition:", error);
        }
      }),
    );

    return { artifactsEmittedPerFile };
  },
);

/**
 * Override task for cleaning up outdated artifacts.
 * Deletes directories with stale `artifacts-nil.d.ts` files that no longer have
 * a matching `.sol` file.
 */
subtask(TASK_COMPILE_REMOVE_OBSOLETE_ARTIFACTS).setAction(
  async (_, { config, artifacts }, runSuper) => {
    const superRes = await runSuper();

    const fqns = await artifacts.getAllFullyQualifiedNames();
    const existingSourceNames = new Set(fqns.map((fqn) => parseFullyQualifiedName(fqn).sourceName));
    const allArtifactsDTs = await getAllFilesMatching(config.paths.artifacts, (f) =>
      f.endsWith("artifacts-nil.d.ts"),
    );

    for (const artifactDTs of allArtifactsDTs) {
      const dir = dirname(artifactDTs);
      const sourceName = replaceBackslashes(relative(config.paths.artifacts, dir));
      // If sourceName is empty, it means that the artifacts.d.ts file is in the
      // root of the artifacts directory, and we shouldn't delete it.
      if (sourceName === "") {
        continue;
      }

      if (!existingSourceNames.has(sourceName)) {
        await rm(dir, { force: true, recursive: true });
      }
    }

    return superRes;
  },
);

const AUTOGENERATED_FILE_PREFACE = `// This file was autogenerated by hardhat-nil, do not edit it.
// prettier-ignore
// tslint:disable
// eslint-disable`;

/**
 * Generates TypeScript code that extends the `ArtifactsMap` with `never` types
 * for duplicate contract names.
 */
function generateDuplicateArtifactsDefinition(duplicateContractNames: Set<string>) {
  return `${AUTOGENERATED_FILE_PREFACE}

import "hardhat/types/artifacts";

declare module "hardhat/types/artifacts" {
  interface ArtifactsNilMap {
    ${Array.from(duplicateContractNames)
      .map((name) => `${name}: never;`)
      .join("\n    ")}
  }

  interface ContractNilTypesMap {
    ${Array.from(duplicateContractNames)
      .map((name) => `${name}: never;`)
      .join("\n    ")}
  }
}
`;
}

/**
 * Generates TypeScript code to declare a contract and its associated
 * TypeScript types.
 */
function generateContractDeclaration(artifact: Artifact, isDuplicate: boolean) {
  const { contractName, sourceName } = artifact;
  const fqn = getFullyQualifiedName(sourceName, contractName);
  const validNames = isDuplicate ? [fqn] : [contractName, fqn];
  const json = JSON.stringify(artifact, undefined, 2);
  const contractTypeName = `${contractName}$NilType`;

  const constructorAbi = artifact.abi.find(({ type }) => type === "constructor");

  const inputs: Array<{
    internalType: string;
    name: string;
    type: string;
  }> = constructorAbi !== undefined ? constructorAbi.inputs : [];

  const constructorArgs =
    inputs.length > 0
      ? `constructorArgs: [${inputs.map(({ name, type }) => getArgType(name, type)).join(", ")}]`
      : `constructorArgs?: []`;

  return `${AUTOGENERATED_FILE_PREFACE}

import type { IAddress, ReadContractsMethod, WriteContractsMethod, ExternalContractsMethod } from "@nilfoundation/niljs";
import "@nilfoundation/hardhat-nil-plugin";
import type { GetContractAtConfig, GetContractAtConfigWithSigner } from "@nilfoundation/hardhat-nil-plugin";
export interface ${contractTypeName} ${json}

declare module "@nilfoundation/hardhat-nil-plugin" {
  ${validNames
    .map(
      (name) => `export function getContractAt(
    contractName: "${name}",
    address: IAddress,
    config?: GetContractAtConfig
  ): Promise<{read: ReadContractsMethod<${contractTypeName}["abi"], "view" | "pure">, write: WriteContractsMethod<${contractTypeName}["abi"], "payable" | "nonpayable">}>;
  export function getContractAt(
    contractName: "${name}",
    address: IAddress,
    config: GetContractAtConfigWithSigner,
  ): Promise<{read: ReadContractsMethod<${contractTypeName}["abi"], "view" | "pure">, write: WriteContractsMethod<${contractTypeName}["abi"], "payable" | "nonpayable">, external: ExternalContractsMethod<${contractTypeName}["abi"], "payable" | "nonpayable", K[]>}>;
  `,
    )
    .join("\n  ")}
}
`;
}

/**
 * Generates TypeScript code to extend the `ArtifactsMap` interface with
 * contract types.
 */
function generateArtifactsDefinition(
  contractTypeData: Array<{
    contractName: string;
    fqn: string;
    typeName: string;
    declaration: string;
  }>,
) {
  return `${AUTOGENERATED_FILE_PREFACE}

import "hardhat/types/artifacts";
import type { IAddress, ReadContractsMethod, WriteContractsMethod } from "@nilfoundation/niljs";

${contractTypeData
  .map((ctd) => `import { ${ctd.typeName} } from "./${ctd.contractName}.nil";`)
  .join("\n")}

declare module "hardhat/types/artifacts" {
  interface ArtifactsNilMap {
    ${contractTypeData.map((ctd) => `["${ctd.contractName}"]: ${ctd.typeName};`).join("\n    ")}
    ${contractTypeData.map((ctd) => `["${ctd.fqn}"]: ${ctd.typeName};`).join("\n    ")}
  }

  interface ContractNilTypesMap {
    ${contractTypeData
      .map(
        (ctd) =>
          `["${ctd.contractName}"]: {read: ReadContractsMethod<${ctd.typeName}["abi"], "view" | "pure">, write: WriteContractsMethod<${ctd.typeName}["abi"], "payable" | "nonpayable">};`,
      )
      .join("\n    ")}
    ${contractTypeData
      .map(
        (ctd) =>
          `["${ctd.fqn}"]: {read: ReadContractsMethod<${ctd.typeName}["abi"], "view" | "pure">, write: WriteContractsMethod<${ctd.typeName}["abi"], "payable" | "nonpayable">};`,
      )
      .join("\n    ")}
  }
}
`;
}

/**
 * Returns the type of a function argument in one of the following formats:
 * - If the 'name' is provided:
 *   "name: AbiParameterToPrimitiveType<{ name: string; type: string; }>"
 *
 * - If the 'name' is empty:
 *   "AbiParameterToPrimitiveType<{ name: string; type: string; }>"
 */
function getArgType(name: string | undefined, type: string) {
  const argType = `AbiParameterToPrimitiveType<${JSON.stringify({
    name,
    type,
  })}>`;

  return name !== "" && name !== undefined ? `${name}: ${argType}` : argType;
}

/**
 * Returns a set of duplicate contract names.
 */
async function findDuplicateContractNames(artifacts: Artifacts) {
  const fqns = await artifacts.getAllFullyQualifiedNames();
  const contractNames = fqns.map((fqn) => parseFullyQualifiedName(fqn).contractName);

  const duplicates = new Set<string>();
  const existing = new Set<string>();

  for (const name of contractNames) {
    if (existing.has(name)) {
      duplicates.add(name);
    }

    existing.add(name);
  }

  return duplicates;
}
