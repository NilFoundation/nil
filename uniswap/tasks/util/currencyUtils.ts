import { shardNumber } from "@nilfoundation/hardhat-plugin/dist/utils/conversion";
import {
  ExternalMessageEnvelope,
  bytesToHex,
  hexToBytes,
  toHex,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { encodeFunctionData } from "viem";

/**
 * Function to mint and send currency from a contract.
 */
export async function mintAndSendCurrency({
  publicClient,
  signer,
  currencyContract,
  contractAddress,
  walletAddress,
  mintAmount,
  hre,
}) {
  const artifact = await hre.artifacts.readArtifact("Currency");
  const chainId = await publicClient.chainId();

  // Mint currency
  let seqNo = await publicClient.getMessageCount(contractAddress, "latest");
  let message = createExternalMessage({
    contractAddress,
    chainId,
    seqNo,
    functionName: "mintCurrency",
    args: [mintAmount],
    abi: artifact.abi,
  });
  message.authData = await message.sign(signer);
  await sendAndAwait(publicClient, walletAddress, message);

  // Sleep to prevent sequence issues
  await sleep(3000);

  // Send currency to wallet
  seqNo = await publicClient.getMessageCount(contractAddress, "latest");
  message = createExternalMessage({
    contractAddress,
    chainId,
    seqNo,
    functionName: "sendCurrency",
    args: [walletAddress, await currencyContract.getCurrencyId(), mintAmount],
    abi: artifact.abi,
  });
  message.authData = await message.sign(signer);
  await sendAndAwait(publicClient, walletAddress, message);
}

/**
 * Helper function to create an ExternalMessageEnvelope.
 */
function createExternalMessage({
  contractAddress,
  chainId,
  seqNo,
  functionName,
  args,
  abi,
}) {
  return new ExternalMessageEnvelope({
    to: hexToBytes(contractAddress),
    chainId,
    seqno: seqNo,
    data: hexToBytes(encodeFunctionData({ abi, functionName, args })),
    authData: new Uint8Array(0),
    isDeploy: false,
  });
}

/**
 * Helper function to send a message and wait for confirmation.
 */
async function sendAndAwait(publicClient, walletAddress, message) {
  const encodedMessage = message.encode();
  await publicClient.sendRawMessage(bytesToHex(encodedMessage));
  await waitTillCompleted(
    publicClient,
    shardNumber(walletAddress),
    toHex(message.hash()),
  );
}

/**
 * Function to withdraw from a faucet to a specific address.
 */
export async function faucetWithdrawal(
  address,
  value,
  faucetAddress,
  hre,
  client,
) {
  const artifact = await hre.artifacts.readArtifact("Faucet");
  const [refinedSeqno, chainId] = await Promise.all([
    client.getMessageCount(faucetAddress, "latest"),
    client.chainId(),
  ]);
  const calldata = encodeFunctionData({
    abi: artifact.abi,
    functionName: "withdrawTo",
    args: [address.toLowerCase(), value],
  });
  const message = new ExternalMessageEnvelope({
    isDeploy: false,
    to: hexToBytes(faucetAddress),
    chainId,
    seqno: refinedSeqno,
    data: hexToBytes(calldata),
    authData: new Uint8Array(0),
  });
  const encodedMessage = message.encode();
  await client.sendRawMessage(bytesToHex(encodedMessage));
  return message.hash();
}

/**
 * Sleep function to pause execution for a specified time (in ms).
 */
export function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
