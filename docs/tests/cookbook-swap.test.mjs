import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  generateRandomPrivateKey,
  convertEthToWei,
  waitTillCompleted,
  hexToBigInt,
  bytesToHex,
  toHex,
  externalDeploymentMessage,
  ExternalMessageEnvelope,
  hexToBytes,
} from "@nilfoundation/niljs";

import {
  encodeFunctionData
} from "viem";

import { NODE_MODULES, RPC_GLOBAL } from "./globals";

const util = require("node:util");
const fs = require("node:fs");
const exec = util.promisify(require("node:child_process").exec);
const RPC_ENDPOINT = RPC_GLOBAL;

beforeAll(async () => {
  await exec(`solc -o ./tests/SwapMatch --abi --bin ./tests/SwapMatch.sol --overwrite ${NODE_MODULES}`);
});

describe.sequential('Nil.js handles the full swap tutorial flow', async () => {
  test.sequential('the Cookbook tutorial flow passes for SwapMatch', async () => {
    //startTwoNewWalletsDeploy
    const SALT = BigInt(Math.floor(Math.random() * 10000));

    const client = new PublicClient({
      transport: new HttpTransport({
        endpoint: RPC_ENDPOINT,
      }),
      shardId: 1,
    });

    const faucet = new Faucet(client);

    const signer = new LocalECDSAKeySigner({
      privateKey: generateRandomPrivateKey(),
    });

    const pubkey = await signer.getPublicKey();

    const walletOne = new WalletV1({
      pubkey: pubkey,
      client,
      signer,
      shardId: 2,
      salt: SALT,
    });

    const walletTwo = new WalletV1({
      pubkey: pubkey,
      client,
      signer,
      shardId: 3,
      salt: SALT
    });

    const walletOneAddress = walletOne.getAddressHex();
    const walletTwoAddress = walletTwo.getAddressHex();

    const fundingWalletOne = await faucet.withdrawToWithRetry(
      walletOneAddress,
      convertEthToWei(10)
    );

    const fundingWalletTwo = await faucet.withdrawToWithRetry(
      walletTwoAddress,
      convertEthToWei(10)
    );

    await walletOne.selfDeploy(true);
    await walletTwo.selfDeploy(true);

    //endTwoNewWalletsDeploy
    expect(walletOneAddress).toBeDefined();
    const walletOneCode = await client.getCode(walletOneAddress, "latest");
    expect(walletOneCode).toBeDefined();
    expect(walletOneCode.length).toBeGreaterThan(10);

    expect(walletTwoAddress).toBeDefined();
    const walletTwoCode = await client.getCode(walletTwoAddress, "latest");
    expect(walletTwoCode).toBeDefined();
    expect(walletTwoCode.length).toBeGreaterThan(10);

    const swapMatchBytecode = "0x60806040525f6002555f6003553480156016575f80fd5b50611176806100245f395ff3fe608060405234801561000f575f80fd5b506004361061003f575f3560e01c8063893e4f4b146100435780639c61d7a41461005f578063f09c582914610093575b5f80fd5b61005d60048036038101906100589190610a84565b6100c8565b005b61007960048036038101906100749190610ac2565b610241565b60405161008a959493929190610b91565b60405180910390f35b6100ad60048036038101906100a89190610ac2565b6102bb565b6040516100bf96959493929190610be2565b60405180910390f35b5f6040518060a001604052803373ffffffffffffffffffffffffffffffffffffffff1681526020016100f8610330565b5f8151811061010a57610109610c41565b5b602002602001015181526020018381526020018481526020015f151581525090508060015f60035481526020019081526020015f205f820151815f015f6101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506020820151816001015f820151815f015560208201518160010155505060408201518160030155606082015181600401556080820151816005015f6101000a81548160ff0219169083151502179055509050503373ffffffffffffffffffffffffffffffffffffffff166003547f76e54bbb791e4aac88689416ddcc3282edd54d5bf521771d25dfaec175ceef2e60405160405180910390a360035f81548092919061022e90610c9b565b919050555061023c816103a9565b505050565b6001602052805f5260405f205f91509050805f015f9054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690806001016040518060400160405290815f820154815260200160018201548152505090806003015490806004015490806005015f9054906101000a900460ff16905085565b5f602052805f5260405f205f91509050805f015f9054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690806001015f9054906101000a900473ffffffffffffffffffffffffffffffffffffffff16908060020154908060030154908060040154908060050154905086565b606060d373ffffffffffffffffffffffffffffffffffffffff16638ee83f196040518163ffffffff1660e01b81526004015f604051808303815f875af115801561037c573d5f803e3d5ffd5b505050506040513d5f823e3d601f19601f820116820180604052508101906103a49190610e97565b905090565b5f5b6003548111610718575f60015f8381526020019081526020015f206040518060a00160405290815f82015f9054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001600182016040518060400160405290815f820154815260200160018201548152505081526020016003820154815260200160048201548152602001600582015f9054906101000a900460ff1615151515815250509050826040015181602001515f01511480156104a25750826060015181602001516020015110155b80156104ba5750806060015183602001516020015110155b80156104cd57505f151581608001511515145b15610704575f604051806040016040528083602001515f01518152602001856060015181525090505f604051806040016040528086602001515f01518152602001846060015181525090506105248584848461071c565b600185608001901515908115158152505060018360800190151590811515815250505f6040518060c00160405280875f015173ffffffffffffffffffffffffffffffffffffffff168152602001855f015173ffffffffffffffffffffffffffffffffffffffff16815260200187602001515f0151815260200185602001515f015181526020018460200151815260200183602001518152509050805f8060025481526020019081526020015f205f820151815f015f6101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506020820151816001015f6101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060408201518160020155606082015181600301556080820151816004015560a08201518160050155905050835f015173ffffffffffffffffffffffffffffffffffffffff16865f015173ffffffffffffffffffffffffffffffffffffffff166002547f4d850df0831c58b3f84efe56818d8cfe1f8a0364bbe9c62c940a2b8a944d29ce60405160405180910390a460025f8154809291906106fb90610c9b565b91905055505050505b50808061071090610c9b565b9150506103ab565b5050565b5f83606001518560200151602001516107359190610ede565b90505f85606001518560200151602001516107509190610ede565b90505f600267ffffffffffffffff81111561076e5761076d610cf6565b5b6040519080825280602002602001820160405280156107a757816020015b610794610a28565b81526020019060019003908161078c5790505b509050845f0151815f815181106107c1576107c0610c41565b5b60200260200101515f0181815250508460200151815f815181106107e8576107e7610c41565b5b6020026020010151602001818152505086602001515f01518160018151811061081457610813610c41565b5b60200260200101515f018181525050828160018151811061083857610837610c41565b5b602002602001015160200181815250505f600267ffffffffffffffff81111561086457610863610cf6565b5b60405190808252806020026020018201604052801561089d57816020015b61088a610a28565b8152602001906001900390816108825790505b509050845f0151815f815181106108b7576108b6610c41565b5b60200260200101515f0181815250508460200151815f815181106108de576108dd610c41565b5b6020026020010151602001818152505086602001515f01518160018151811061090a57610909610c41565b5b60200260200101515f018181525050828160018151811061092e5761092d610c41565b5b60200260200101516020018181525050610963885f0151895f0151305f805f808960405180602001604052805f815250610992565b610988875f0151885f0151305f805f808860405180602001604052805f815250610992565b5050505050505050565b60fd73ffffffffffffffffffffffffffffffffffffffff1663f1d1cf1c8486888d8d8d8d8a8a6040518a63ffffffff1660e01b81526004016109db989796959493929190611061565b60206040518083038185885af11580156109f7573d5f803e3d5ffd5b50505050506040513d601f19601f82011682018060405250810190610a1c9190611115565b50505050505050505050565b60405180604001604052805f81526020015f81525090565b5f604051905090565b5f80fd5b5f80fd5b5f819050919050565b610a6381610a51565b8114610a6d575f80fd5b50565b5f81359050610a7e81610a5a565b92915050565b5f8060408385031215610a9a57610a99610a49565b5b5f610aa785828601610a70565b9250506020610ab885828601610a70565b9150509250929050565b5f60208284031215610ad757610ad6610a49565b5b5f610ae484828501610a70565b91505092915050565b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f610b1682610aed565b9050919050565b610b2681610b0c565b82525050565b610b3581610a51565b82525050565b604082015f820151610b4f5f850182610b2c565b506020820151610b626020850182610b2c565b50505050565b610b7181610a51565b82525050565b5f8115159050919050565b610b8b81610b77565b82525050565b5f60c082019050610ba45f830188610b1d565b610bb16020830187610b3b565b610bbe6060830186610b68565b610bcb6080830185610b68565b610bd860a0830184610b82565b9695505050505050565b5f60c082019050610bf55f830189610b1d565b610c026020830188610b1d565b610c0f6040830187610b68565b610c1c6060830186610b68565b610c296080830185610b68565b610c3660a0830184610b68565b979650505050505050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52603260045260245ffd5b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f610ca582610a51565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8203610cd757610cd6610c6e565b5b600182019050919050565b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b610d2c82610ce6565b810181811067ffffffffffffffff82111715610d4b57610d4a610cf6565b5b80604052505050565b5f610d5d610a40565b9050610d698282610d23565b919050565b5f67ffffffffffffffff821115610d8857610d87610cf6565b5b602082029050602081019050919050565b5f80fd5b5f80fd5b5f81519050610daf81610a5a565b92915050565b5f60408284031215610dca57610dc9610d9d565b5b610dd46040610d54565b90505f610de384828501610da1565b5f830152506020610df684828501610da1565b60208301525092915050565b5f610e14610e0f84610d6e565b610d54565b90508083825260208201905060408402830185811115610e3757610e36610d99565b5b835b81811015610e605780610e4c8882610db5565b845260208401935050604081019050610e39565b5050509392505050565b5f82601f830112610e7e57610e7d610ce2565b5b8151610e8e848260208601610e02565b91505092915050565b5f60208284031215610eac57610eab610a49565b5b5f82015167ffffffffffffffff811115610ec957610ec8610a4d565b5b610ed584828501610e6a565b91505092915050565b5f610ee882610a51565b9150610ef383610a51565b9250828203905081811115610f0b57610f0a610c6e565b5b92915050565b5f60ff82169050919050565b610f2681610f11565b82525050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b604082015f820151610f695f850182610b2c565b506020820151610f7c6020850182610b2c565b50505050565b5f610f8d8383610f55565b60408301905092915050565b5f602082019050919050565b5f610faf82610f2c565b610fb98185610f36565b9350610fc483610f46565b805f5b83811015610ff4578151610fdb8882610f82565b9750610fe683610f99565b925050600181019050610fc7565b5085935050505092915050565b5f81519050919050565b5f82825260208201905092915050565b8281835e5f83830152505050565b5f61103382611001565b61103d818561100b565b935061104d81856020860161101b565b61105681610ce6565b840191505092915050565b5f610100820190506110755f83018b610b82565b611082602083018a610f1d565b61108f6040830189610b1d565b61109c6060830188610b1d565b6110a96080830187610b1d565b6110b660a0830186610b68565b81810360c08301526110c88185610fa5565b905081810360e08301526110dc8184611029565b90509998505050505050505050565b6110f481610b77565b81146110fe575f80fd5b50565b5f8151905061110f816110eb565b92915050565b5f6020828403121561112a57611129610a49565b5b5f61113784828501611101565b9150509291505056fea26469706673582212202418f4215f5066aeb5a609960e97d1cdc479aa84b70a3f80251f07f4cd59aba964736f6c634300081a0033";
    const swapMatchAbi = [{ "anonymous": false, "inputs": [{ "indexed": true, "internalType": "uint256", "name": "swapNumber", "type": "uint256" }, { "indexed": true, "internalType": "address", "name": "", "type": "address" }, { "indexed": true, "internalType": "address", "name": "", "type": "address" }], "name": "NewSwap", "type": "event" }, { "anonymous": false, "inputs": [{ "indexed": true, "internalType": "uint256", "name": "swapRequestNumber", "type": "uint256" }, { "indexed": true, "internalType": "address", "name": "", "type": "address" }], "name": "NewSwapRequest", "type": "event" }, { "anonymous": false, "inputs": [{ "indexed": true, "internalType": "uint256", "name": "swapNumber", "type": "uint256" }, { "indexed": true, "internalType": "address", "name": "", "type": "address" }, { "indexed": true, "internalType": "address", "name": "", "type": "address" }], "name": "SwapFinalized", "type": "event" }, { "inputs": [{ "internalType": "uint256", "name": "_desiredSecondTokenAmount", "type": "uint256" }, { "internalType": "uint256", "name": "_secondTokenId", "type": "uint256" }], "name": "placeSwapRequest", "outputs": [], "stateMutability": "nonpayable", "type": "function" }, { "inputs": [{ "internalType": "uint256", "name": "", "type": "uint256" }], "name": "swapRequests", "outputs": [{ "internalType": "address", "name": "initiator", "type": "address" }, { "components": [{ "internalType": "uint256", "name": "id", "type": "uint256" }, { "internalType": "uint256", "name": "amount", "type": "uint256" }], "internalType": "struct Nil.Token", "name": "token", "type": "tuple" }, { "internalType": "uint256", "name": "secondTokenId", "type": "uint256" }, { "internalType": "uint256", "name": "desiredSecondTokenAmount", "type": "uint256" }, { "internalType": "bool", "name": "isMatched", "type": "bool" }], "stateMutability": "view", "type": "function" }, { "inputs": [{ "internalType": "uint256", "name": "", "type": "uint256" }], "name": "swaps", "outputs": [{ "internalType": "address", "name": "firstParty", "type": "address" }, { "internalType": "address", "name": "secondParty", "type": "address" }, { "internalType": "uint256", "name": "firstTokenId", "type": "uint256" }, { "internalType": "uint256", "name": "secondTokenId", "type": "uint256" }, { "internalType": "uint256", "name": "firstTokenExchanged", "type": "uint256" }, { "internalType": "uint256", "name": "secondTokenExchanged", "type": "uint256" }], "stateMutability": "view", "type": "function" }];
    //startDeploymentOfSwapMatch
    const wallet = new WalletV1({
      pubkey: pubkey,
      client,
      signer,
      shardId: 1,
      salt: SALT,
    });

    await faucet.withdrawToWithRetry(wallet.getAddressHex(), 300_000_000n);

    await wallet.selfDeploy(true);

    const { address: swapMatchAddress, hash: deploymentMessageHash } = await wallet.deployContract({
      bytecode: swapMatchBytecode,
      value: 0n,
      feeCredit: 100_000_000n,
      salt: SALT,
      shardId: 4
    });

    const receipts = await waitTillCompleted(client, 1, deploymentMessageHash);
    //endDeploymentOfSwapMatch

    expect(receipts.some((receipt) => !receipt.success)).toBe(false);

    const code = await client.getCode(swapMatchAddress, "latest");

    expect(code).toBeDefined();
    expect(code.length).toBeGreaterThan(10);

    //startCurrencyCreation
    {
      const hashMessage = await walletOne.mintCurrency(100_000_000n);
      await waitTillCompleted(client, 2, hashMessage);
    }

    {
      const hashMessage = await walletTwo.mintCurrency(100_000_000n);
      await waitTillCompleted(client, 3, hashMessage);
    }
    //endCurrencyCreation

    //startFirstSendRequest
    {
      const gasPrice = await client.getGasPrice(2);
      const hashMessage = await walletOne.sendMessage({
        to: swapMatchAddress,
        tokens: [
          {
            id: hexToBigInt(walletOneAddress),
            amount: 30_000_000n
          }
        ],
        abi: swapMatchAbi,
        functionName: 'placeSwapRequest',
        args: [
          20_000_000n,
          hexToBigInt(walletTwoAddress)
        ],
        feeCredit: gasPrice * 1_000_000_000n,

      });

      await waitTillCompleted(client, 2, hashMessage);
    }
    //endFirstSendRequest

    //startSecondSendRequest
    {
      const gasPrice = await client.getGasPrice(3);
      const hashMessage = await walletTwo.sendMessage({
        to: swapMatchAddress,
        tokens: [
          {
            id: hexToBigInt(walletTwoAddress),
            amount: 50_000_000n
          }
        ],
        abi: swapMatchAbi,
        functionName: 'placeSwapRequest',
        args: [
          10_000_000n,
          hexToBigInt(walletOneAddress)
        ],
        feeCredit: gasPrice * 1_000_000_000n,

      });

      await waitTillCompleted(client, 3, hashMessage);
    }

    //endSecondSendRequest

    //startFinalChecks
    const tokensOne = await client.getCurrencies(walletOneAddress, "latest");
    const tokensTwo = await client.getCurrencies(walletTwoAddress, "latest");
    console.log('Wallet 1 tokens: ', tokensOne);
    console.log('Wallet 2 tokens: ', tokensTwo);
    //endFinalChecks

    const amountsOne = Object.values(tokensOne);
    const expectedAmountsOne = [90_000_000n, 20_000_000n];

    amountsOne.sort();
    expectedAmountsOne.sort();

    expect(amountsOne).to.deep.equal(expectedAmountsOne);

    const amountsTwo = Object.values(tokensTwo);
    const expectedAmountsTwo = [10_000_000n, 80_000_000n];
    amountsTwo.sort();
    expectedAmountsTwo.sort();
    expect(amountsTwo).to.deep.equal(expectedAmountsTwo);


  }, 70000);

});