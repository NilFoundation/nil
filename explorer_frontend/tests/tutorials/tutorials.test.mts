
import type { CheckProps } from "../../src/features/tutorial-check/CheckProps";
import runTutorialCheckOne from "../../src/features/tutorial-check/checks/tutorialOneCheck";
import runTutorialCheckΤwo from "../../src/features/tutorial-check/checks/tutorialTwoCheck";
import runTutorialCheckThree from "../../src/features/tutorial-check/checks/tutorialThreeCheck";
import runTutorialCheckFour from "../../src/features/tutorial-check/checks/tutorialFourCheck";
import { deploymentEffect, RPC_ENDPOINT, setCompletedTutorialEvent, setTutorialChecksEvent, tutorialContractStepFailed, tutorialContractStepPassed } from "./globals";
import { expect, describe, test } from "vitest";
const solc = require("solc");
import path from "node:path";
import { createCompileInput } from "../../src/features/shared/utils/solidityCompiler/helper.ts";
import type { App } from "../../src/features/code/types.ts";
import runTutorialCheckFive from "../../src/features/tutorial-check/checks/tutorialFiveCheck.ts";

const TEST_PROPS: CheckProps = {
  rpcUrl: RPC_ENDPOINT,
  deploymentEffect: deploymentEffect,
  setTutorialChecksEvent: setTutorialChecksEvent,
  tutorialContractStepFailed: tutorialContractStepFailed,
  tutorialContractStepPassed: tutorialContractStepPassed,
  contracts: [],
  setCompletedTutorialEvent: setCompletedTutorialEvent
};

const createContracts = async (code: string) => {
  const input = createCompileInput(code);
  const res = JSON.parse(solc.compile(JSON.stringify(input)));

  const contracts: App[] = [];
  if ("contracts" in res && res.contracts !== undefined && "Compiled_Contracts" in res.contracts) {
    for (const name in res.contracts?.Compiled_Contracts) {
      const contract = res.contracts.Compiled_Contracts[name];

      contracts.push({
        name: name,
        bytecode: `0x${contract.evm.bytecode.object}`,
        sourcecode: code,
        abi: contract.abi,
      });
    }
  }

  return contracts;
}


describe("Tutorial One tests", async () => {
  test("Tutorial One passes with the given solution", async () => {
    const code = await import(path.resolve(__dirname, "../../src/features/tutorial/assets/tutorialOne/tutorialOneSolution.sol?raw"));
    const codeRes = code.default;

    const contracts = await createContracts(codeRes);
    const testProps: CheckProps = { ...TEST_PROPS, contracts };
    const testRes = await runTutorialCheckOne(testProps);
    expect(testRes).toBe(true);
  })
}, 40000);

describe("Tutorial Two tests", async () => {
  test("Tutorial Two passes with the given solution", async () => {
    const code = await import(path.resolve(__dirname, "../../src/features/tutorial/assets/tutorialTwo/tutorialTwoSolution.sol?raw"));
    const codeRes = code.default;

    const contracts = await createContracts(codeRes);

    const testProps: CheckProps = { ...TEST_PROPS, contracts };


    const testRes = await runTutorialCheckΤwo(testProps);
    expect(testRes).toBe(true);
  })
}, 45000);

describe("Tutorial Three tests", async () => {
  test("Tutorial Three passes with the given solution", async () => {
    const code = await import(path.resolve(__dirname, "../../src/features/tutorial/assets/tutorialThree/tutorialThreeSolution.sol?raw"));
    const codeRes = code.default;

    const contracts = await createContracts(codeRes);

    const testProps: CheckProps = { ...TEST_PROPS, contracts };

    const testRes = await runTutorialCheckThree(testProps);
    expect(testRes).toBe(true);
  })
}, 45000);

describe("Tutorial Four tests", async () => {
  test("Tutorial Four passes with the given solution", async () => {
    const code = await import(path.resolve(__dirname, "../../src/features/tutorial/assets/tutorialFour/tutorialFourSolution.sol?raw"));
    const codeRes = code.default;

    const contracts = await createContracts(codeRes);

    const testProps: CheckProps = { ...TEST_PROPS, contracts };

    const testRes = await runTutorialCheckFour(testProps);
    expect(testRes).toBe(true);
  })
}, 45000);

describe("Tutorial Five tests", async () => {
  test("Tutorial Five passes with the given solution", async () => {
    const code = await import(path.resolve(__dirname, "../../src/features/tutorial/assets/tutorialFive/tutorialFiveSolution.sol?raw"));
    const codeRes = code.default;

    const contracts = await createContracts(codeRes);

    const testProps: CheckProps = { ...TEST_PROPS, contracts };

    const testRes = await runTutorialCheckFive(testProps);
    expect(testRes).toBe(true);
  });
}, 45000);



