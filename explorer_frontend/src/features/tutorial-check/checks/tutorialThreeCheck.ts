import { HttpTransport, PublicClient, generateSmartAccount } from "@nilfoundation/niljs";
import { TutorialChecksStatus } from "../../../pages/tutorials/model";
import { deploySmartContractFx } from "../../contracts/models/base";
import type { CheckProps } from "../CheckProps";

async function runTutorialCheckThree(props: CheckProps) {
  const client = new PublicClient({
    transport: new HttpTransport({
      endpoint: props.rpcUrl,
    }),
    shardId: 1,
  });

  const requesterContract = props.contracts.find((contract) => contract.name === "Requester")!;
  const requestedContract = props.contracts.find(
    (contract) => contract.name === "RequestedContract",
  )!;

  const appRequester = {
    name: "Requester",
    bytecode: requesterContract.bytecode,
    abi: requesterContract.abi,
    sourcecode: requesterContract.sourcecode,
  };

  const appRequestedContract = {
    name: "RequestedContract",
    bytecode: requestedContract.bytecode,
    abi: requestedContract.abi,
    sourcecode: requestedContract.sourcecode,
  };

  const smartAccount = await generateSmartAccount({
    shardId: 1,
    rpcEndpoint: props.rpcUrl,
    faucetEndpoint: props.rpcUrl,
  });

  props.tutorialContractStepPassed("A new smart account has been generated!");

  const resultRequester = await props.deploymentEffect({
    app: appRequester,
    args: [],
    shardId: 1,
    smartAccount,
  });

  const resultRequestedContract = await deploySmartContractFx({
    app: appRequestedContract,
    args: [],
    shardId: 2,
    smartAccount,
  });

  props.tutorialContractStepPassed("Requester and RequestedContract have been deployed!");

  const requestTx = await smartAccount.sendTransaction({
    to: resultRequester.address,
    abi: requesterContract.abi,
    functionName: "requestMultiplication",
    args: [resultRequestedContract.address],
  });

  const resRequest = await requestTx.wait();

  const checkRequest = await resRequest.some((receipt) => !receipt.success);

  if (checkRequest) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    console.log(resRequest);
    props.tutorialContractStepFailed(
      `
      Calling Requester.requestMultiplication() produced one or more failed receipts!
      To investigate, debug this transaction using the Cometa service: ${hashRequest}.
      `,
    );
    return false;
  }

  props.tutorialContractStepPassed(
    "Requester.requestMultiplication() has been called successfully!",
  );

  const result = await client.call(
    {
      to: resultRequester.address,
      abi: requesterContract.abi,
      functionName: "getResult",
    },
    "latest",
  );

  if (!result.decodedData) {
    props.tutorialContractStepFailed("Failed to verify the result of multiplication!");
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    return false;
  }

  props.tutorialContractStepPassed("The result of multiplication has been verified!");

  props.setTutorialChecksEvent(TutorialChecksStatus.Successful);

  props.setCompletedTutorialEvent(3);

  props.tutorialContractStepPassed("Tutorial Three has been completed successfully!");

  return true;
}

export default runTutorialCheckThree;
