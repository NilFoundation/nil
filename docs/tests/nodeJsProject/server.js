//startImport
const { createServer } = require("node:http");
const {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  generateRandomPrivateKey,
  convertEthToWei,
} = require("@nilfoundation/niljs");

const hostname = "127.0.0.1";
const port = 3000;
//endImport

const RPC_ENDPOINT = "http://127.0.0.1:8529";

//startServer
const server = createServer((req, res) => {
  (async () => {
    try {
      res.statusCode = 200;
      res.setHeader("Content-Type", "text/plain");
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

      const pubkey = signer.getPublicKey();
      const wallet = new WalletV1({
        pubkey: pubkey,
        salt: BigInt(Math.floor(Math.random() * 10000)),
        shardId: 1,
        client,
        signer,
      });

      const walletAddress = wallet.address;
      await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(1));
      await wallet.selfDeploy(true);

      res.write(`New wallet address: ${walletAddress}\n`);
      res.on("finish", () => {
        console.log(`New wallet address: ${walletAddress}`);
      });
      res.end();
    } catch (error) {
      console.error(error);
      res.statusCode = 500;
      res.write("An error occurred while creating the wallet.");
      res.end();
    }
  })();
});

server.listen(port, hostname, () => {
  console.log(`Server running at http://${hostname}:${port}/`);
});
//endServer
