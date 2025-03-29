import {
  HttpTransport,
  PublicClient,
  generateSmartAccount,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { TutorialChecksStatus } from "../../../pages/tutorials/model";
import type { CheckProps } from "../CheckProps";

const CUSTOM_TOKEN_AMOUNT = 30_000n;

async function runTutorialCheckTwo(props: CheckProps) {
  const client = new PublicClient({
    transport: new HttpTransport({
      endpoint: props.rpcUrl,
    }),
    shardId: 1,
  });

  const operatorContract = props.contracts.find((contract) => contract.name === "Operator")!;

  const customTokenContract = props.contracts.find((contract) => contract.name === "CustomToken")!;

  const appOperator = {
    name: "Operator",
    bytecode: operatorContract.bytecode,
    abi: operatorContract.abi,
    sourcecode: operatorContract.sourcecode,
  };

  const appCustomToken = {
    name: "CustomToken",
    bytecode: customTokenContract.bytecode,
    abi: customTokenContract.abi,
    sourcecode: customTokenContract.sourcecode,
  };

  const smartAccount = await generateSmartAccount({
    shardId: 1,
    rpcEndpoint: props.rpcUrl,
    faucetEndpoint: props.rpcUrl,
  });

  props.tutorialContractStepPassed("A new smart account has been generated!");

  const resultOperator = await props.deploymentEffect({
    app: appOperator,
    args: [],
    shardId: 1,
    smartAccount,
  });

  const resultCustomToken = await props.deploymentEffect({
    app: appCustomToken,
    args: [resultOperator.address],
    shardId: 2,
    smartAccount,
  });

  props.tutorialContractStepPassed("Operator and CustomToken have been deployed!");

  const hashMinting = await smartAccount.sendTransaction({
    to: resultOperator.address,
    abi: operatorContract.abi,
    functionName: "checkMintToken",
    args: [resultCustomToken.address, CUSTOM_TOKEN_AMOUNT],
  });

  const resMinting = await waitTillCompleted(client, hashMinting);

  const checkMinting = resMinting.some((receipt) => !receipt.success);

  if (checkMinting) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    console.log(resMinting);
    props.tutorialContractStepFailed("Failed to call mintTokenCustom()!");
    return false;
  }

  props.tutorialContractStepPassed("mintTokenCustom() has been called successfully!");

  const customTokenBalance = await client.getTokens(resultCustomToken.address, "latest");

  if (Object.values(customTokenBalance).at(0) !== CUSTOM_TOKEN_AMOUNT) {
    console.log(customTokenBalance);
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    props.tutorialContractStepFailed("CustomToken failed to mint tokens!");
    return false;
  }

  props.tutorialContractStepPassed("CustomToken has minted tokens successfully!");

  const gasPrice = await client.getGasPrice(1);

  const hashSending = await smartAccount.sendTransaction({
    to: resultOperator.address,
    abi: operatorContract.abi,
    functionName: "checkSendToken",
    args: [resultCustomToken.address, CUSTOM_TOKEN_AMOUNT],
    feeCredit: gasPrice * 5_000_000n,
  });

  const resSending = await waitTillCompleted(client, hashSending);

  const checkSending = resSending.some((receipt) => !receipt.success);

  if (checkSending) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    console.log(resSending);
    props.tutorialContractStepFailed("Failed to call sendTokenCustom()!");
    return false;
  }

  props.tutorialContractStepPassed("sendTokenCustom() has been called successfully!");

  const customTokenBalanceOperator = await client.getTokens(resultOperator.address, "latest");

  if (Object.values(customTokenBalanceOperator).at(1) === CUSTOM_TOKEN_AMOUNT) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    props.tutorialContractStepFailed("Operator did not receive tokens from CustomToken!");
    return false;
  }

  props.tutorialContractStepPassed("Operator has received tokens from CustomToken successfully!");
  props.setTutorialChecksEvent(TutorialChecksStatus.Successful);
  props.tutorialContractStepPassed("Tutorial has been completed successfully!");

  props.setCompletedTutorialEvent(2);

  return true;
}

export default runTutorialCheckTwo;
