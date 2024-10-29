import { sample } from "effector";
import { $faucets, fetchFaucetsEvent, fetchFaucetsFx } from "./model";
import { FaucetClient, HttpTransport } from "@nilfoundation/niljs";
import { $endpoint } from "../account-connector/models/model";

fetchFaucetsFx.use(async (endpoint) => {
  const faucetClient = new FaucetClient({
    transport: new HttpTransport({ endpoint }),
  });

  const faucets = await faucetClient.getAllFaucets();

  return faucets;
});

sample({
  clock: fetchFaucetsEvent,
  source: $endpoint,
  target: fetchFaucetsFx,
});

$endpoint.watch((endpoint) => {
  if (endpoint) {
    fetchFaucetsEvent();
  }
});

$faucets.on(fetchFaucetsFx.doneData, (_, balance) => balance);

fetchFaucetsEvent();
