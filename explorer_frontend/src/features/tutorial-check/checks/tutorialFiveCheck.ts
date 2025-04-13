import { HttpTransport, PublicClient, generateSmartAccount } from "@nilfoundation/niljs";
import { TutorialChecksStatus } from "../../../pages/tutorials/model";
import type { CheckProps } from "../CheckProps";

async function runTutorialCheckFive(props: CheckProps) {
  const client = new PublicClient({
    transport: new HttpTransport({
      endpoint: props.rpcUrl,
    }),
    shardId: 1,
  });

  const gasPrice = await client.getGasPrice(1);

  const receiverContract = props.contracts.find((contract) => contract.name === "Receiver")!;
  const NFTContract = props.contracts.find((contract) => contract.name === "NFT")!;

  const appReceiver = {
    name: "Receiver",
    bytecode: receiverContract.bytecode,
    abi: receiverContract.abi,
    sourcecode: receiverContract.sourcecode,
  };

  const appNFT = {
    name: "NFT",
    bytecode: NFTContract.bytecode,
    abi: NFTContract.abi,
    sourcecode: NFTContract.sourcecode,
  };

  const smartAccount = await generateSmartAccount({
    shardId: 1,
    rpcEndpoint: props.rpcUrl,
    faucetEndpoint: props.rpcUrl,
  });

  props.tutorialContractStepPassed("A new smart account has been generated!");

  const resultReceiver = await props.deploymentEffect({
    app: appReceiver,
    args: [],
    shardId: 1,
    smartAccount,
  });

  const resultNFT = await props.deploymentEffect({
    app: appNFT,
    args: [],
    shardId: 2,
    smartAccount,
  });

  props.tutorialContractStepPassed("Receiver and NFT have been deployed successfully!");

  const mintRequest = await smartAccount.sendTransaction({
    to: resultNFT.address,
    abi: NFTContract.abi,
    functionName: "mintNFT",
    args: [],
    feeCredit: gasPrice * 500_000n,
  });

  const resMinting = await mintRequest.wait();

  const checkMinting = await resMinting.some((receipt) => !receipt.success);

  if (checkMinting) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    console.log(resMinting);
    props.tutorialContractStepFailed(
      `
      Calling NFT.mintNFT() produced one or more failed receipts!
      To investigate, debug this transaction using the Cometa service: ${mintRequest}.
      `,
    );
    return false;
  }

  props.tutorialContractStepPassed("NFT has been minted successfully!");

  const secondMintRequest = await smartAccount.sendTransaction({
    to: resultNFT.address,
    abi: NFTContract.abi,
    functionName: "mintNFT",
    args: [],
    feeCredit: gasPrice * 500_000n,
  });

  const resSecondMinting = await secondMintRequest.wait();

  const checkSecondMinting = await resSecondMinting.some((receipt) => !receipt.success);

  if (!checkSecondMinting) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    console.log(resSecondMinting);
    props.tutorialContractStepFailed(
      `
      Calling NFT.mintNFT() the second time did not produce any failed receipts!
      The NFT has been minted twice.
      `,
    );
    return false;
  }

  props.tutorialContractStepPassed("NFT is protected against repeated minting!");

  const sendRequest = await smartAccount.sendTransaction({
    to: resultNFT.address,
    abi: NFTContract.abi,
    functionName: "sendNFT",
    args: [resultReceiver.address],
  });

  const resSending = await sendRequest.wait();

  const checkSending = await resSending.some((receipt) => !receipt.success);

  if (checkSending) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    console.log(resSending);
    props.tutorialContractStepFailed(
      `
      Calling NFT.sendNFT() produced one or more failed receipts!
      To investigate, debug this transaction using the Cometa service: ${sendRequest}.
      `,
    );
    return false;
  }

  const result = await client.getTokens(resultReceiver.address, "latest");

  if (Object.keys(result).length === 0) {
    props.setTutorialChecksEvent(TutorialChecksStatus.Failed);
    props.tutorialContractStepFailed("NFT has not been received!");
    return false;
  }

  props.tutorialContractStepPassed("NFT has been received successfully!");

  props.setTutorialChecksEvent(TutorialChecksStatus.Successful);

  props.tutorialContractStepPassed("Tutorial has been completed successfully!");

  props.setCompletedTutorialEvent(5);

  return true;
}

export default runTutorialCheckFive;
