import { HttpTransport, PublicClient, waitTillCompleted } from "@nilfoundation/niljs";
import {
  $tutorialChecksState,
  TutorialChecksStatus,
  setTutorialChecksState,
} from "../../../pages/tutorials/model";
import type { App } from "../../../types";
import { $rpcUrl, $smartAccount } from "../../account-connector/model";
import { $contracts, deploySmartContractFx } from "../../contracts/models/base";
import { tutorialContractStepFailedEvent, tutorialContractStepPassedEvent } from "../model";

async function runTutorialCheckOne() {
  const client = new PublicClient({
    transport: new HttpTransport({
      endpoint: $rpcUrl.getState(),
    }),
    shardId: 1,
  });

  const callerContract = $contracts.getState().find((contract) => contract.name === "Caller")!;

  const receiverContract = $contracts.getState().find((contract) => contract.name === "Receiver")!;

  const appCaller: App = {
    name: "Caller",
    bytecode: callerContract.bytecode,
    abi: callerContract.abi,
    sourcecode: callerContract.sourcecode,
  };

  console.log("appCaller", appCaller.sourcecode);

  const appReceiver: App = {
    name: "Receiver",
    bytecode: receiverContract.bytecode,
    abi: receiverContract.abi,
    sourcecode: receiverContract.sourcecode,
  };

  console.log("appReceiver", appReceiver.sourcecode);

  const smartAccount = $smartAccount.getState()!;

  const resultCaller = await deploySmartContractFx({
    app: appCaller,
    args: [],
    shardId: 1,
    smartAccount,
  });

  const resultReceiver = await deploySmartContractFx({
    app: appReceiver,
    args: [],
    shardId: 2,
    smartAccount,
  });

  tutorialContractStepPassedEvent("Caller and Receiver have been deployed!");

  const hashCaller = await smartAccount.sendTransaction({
    to: resultCaller.address,
    abi: callerContract.abi,
    functionName: "sendValue",
    args: [resultReceiver.address],
    value: 500_000n,
  });

  const resCaller = await waitTillCompleted(client, hashCaller);

  const checkCaller = resCaller.some((receipt) => !receipt.success);

  if (checkCaller) {
    setTutorialChecksState(TutorialChecksStatus.Failed);
    console.log("current state", $tutorialChecksState.getState());
    tutorialContractStepFailedEvent("Failed to call Caller.sendValue()!");
    return;
  }
  tutorialContractStepPassedEvent("Caller sendValue has been called successfully!");

  const receiverBalance = await client.getBalance(resultReceiver.address);

  if (receiverBalance !== 300_000n) {
    setTutorialChecksState(TutorialChecksStatus.Failed);
    tutorialContractStepFailedEvent("Receiver failed to receive tokens!");
    return;
  }
  tutorialContractStepPassedEvent("Receiver got 300_000 tokens!");
  setTutorialChecksState(TutorialChecksStatus.Successful);
  tutorialContractStepPassedEvent("Tutorial has been completed successfully!");
}

export default runTutorialCheckOne;
