import {
  HttpTransport,
  PublicClient,
  generateSmartAccount,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { TutorialChecksStatus } from "../../../pages/tutorials/model";
import type { CheckProps } from "../CheckProps";
import {} from "../model";

async function runTutorialCheckOne(props: CheckProps) {
  const client = new PublicClient({
    transport: new HttpTransport({
      endpoint: props.rpcUrl,
    }),
    shardId: 1,
  });

  const callerContract = props.contracts.find((contract) => contract.name === "Caller")!;

  const receiverContract = props.contracts.find((contract) => contract.name === "Receiver")!;

  const appCaller = {
    name: "Caller",
    bytecode: callerContract.bytecode,
    abi: callerContract.abi,
    sourcecode: callerContract.sourcecode,
  };

  const appReceiver = {
    name: "Receiver",
    bytecode: receiverContract.bytecode,
    abi: receiverContract.abi,
    sourcecode: receiverContract.sourcecode,
  };

  const smartAccount = await generateSmartAccount({
    shardId: 1,
    rpcEndpoint: props.rpcUrl,
    faucetEndpoint: props.rpcUrl,
  });

  props.tutorialContractStepPassed("A new smart account has been generated!");

  const resultCaller = await props.deploymentEffect({
    app: appCaller,
    args: [],
    shardId: 1,
    smartAccount,
  });

  const resultReceiver = await props.deploymentEffect({
    app: appReceiver,
    args: [],
    shardId: 2,
    smartAccount,
  });

  props.tutorialContractStepPassed("Caller and Receiver have been deployed!");

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
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    console.log(resCaller);
    props.tutorialContractStepFailed("Failed to call Caller.sendValue()!");
    return false;
  }
  props.tutorialContractStepPassed("Caller sendValue has been called successfully!");

  const receiverBalance = await client.getBalance(resultReceiver.address);

  if (receiverBalance !== 300_000n) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    props.tutorialContractStepFailed("Receiver failed to receive tokens!");
    return false;
  }
  props.tutorialContractStepPassed("Receiver got 300_000 tokens!");
  props.setTutorialChecksEvent(TutorialChecksStatus.Successful);
  props.tutorialContractStepPassed("Tutorial has been completed successfully!");

  props.setCompletedTutorialEvent(1);

  return true;
}

export default runTutorialCheckOne;
