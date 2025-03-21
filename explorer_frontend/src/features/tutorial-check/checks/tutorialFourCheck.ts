import {
  HttpTransport,
  PublicClient,
  generateSmartAccount,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { TutorialChecksStatus } from "../../../pages/tutorials/model";
import type { CheckProps } from "../CheckProps";

async function runTutorialCheckFour(props: CheckProps) {
  const client = new PublicClient({
    transport: new HttpTransport({
      endpoint: props.rpcUrl,
    }),
    shardId: 1,
  });

  const counterContract = props.contracts.find((contract) => contract.name === "Counter")!;
  const deployerContract = props.contracts.find((contract) => contract.name === "Deployer")!;

  const appCounter = {
    name: "Counter",
    bytecode: counterContract.bytecode,
    abi: counterContract.abi,
    sourcecode: counterContract.sourcecode,
  };

  const appDeployer = {
    name: "Deployer",
    bytecode: deployerContract.bytecode,
    abi: deployerContract.abi,
    sourcecode: deployerContract.sourcecode,
  };

  const smartAccount = await generateSmartAccount({
    shardId: 1,
    rpcEndpoint: props.rpcUrl,
    faucetEndpoint: props.rpcUrl,
  });

  props.tutorialContractStepPassed("A new smart account has been generated!");

  const resultDeployer = await props.deploymentEffect({
    app: appDeployer,
    args: [],
    shardId: 2,
    smartAccount,
  });

  props.tutorialContractStepPassed("Deployer has been deployed!");

  const gasPrice = await client.getGasPrice(1);

  const salt = BigInt(Math.floor(Math.random() * 1000000));

  const hashDeploy = await smartAccount.sendTransaction({
    to: resultDeployer.address,
    abi: deployerContract.abi,
    functionName: "deploy",
    args: [appCounter.bytecode, salt],
    feeCredit: gasPrice * 500_000n,
  });

  const resDeploy = await waitTillCompleted(client, hashDeploy);

  const checkDeploy = await resDeploy.some((receipt) => !receipt.success);

  if (checkDeploy) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    console.log(resDeploy);
    props.tutorialContractStepFailed("Failed to call Deployer.deploy()!");
    return false;
  }

  props.tutorialContractStepPassed("Counter has been deployed!");

  const counterAddress = resDeploy.at(2)?.contractAddress as `0x${string}`;

  const hashIncrement = await smartAccount.sendTransaction({
    to: counterAddress,
    abi: counterContract.abi,
    functionName: "increment",
    args: [],
    feeCredit: gasPrice * 500_000n,
  });

  const resIncrement = await waitTillCompleted(client, hashIncrement);

  const checkIncrement = resIncrement.some((receipt) => !receipt.success);

  if (checkIncrement) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    console.log(resIncrement);
    props.tutorialContractStepFailed("Failed to call Counter.increment()!");
    return false;
  }

  props.tutorialContractStepPassed("Counter.increment() has been called successfully!");

  props.setTutorialChecksEvent(TutorialChecksStatus.Successful);

  props.tutorialContractStepPassed("Tutorial has been completed successfully!");

  props.setCompletedTutorialEvent(3);

  return true;
}

export default runTutorialCheckFour;
