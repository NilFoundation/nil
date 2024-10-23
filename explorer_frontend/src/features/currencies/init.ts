import { sample } from "effector";
import { $faucets, $faucetsEndpoint, fetchFaucetsEvent, fetchFaucetsFx } from "./model";
import { FaucetClient, HttpTransport } from "@nilfoundation/niljs";

fetchFaucetsFx.use(async (endpoint) => {
  const faucetClient = new FaucetClient({
    transport: new HttpTransport({ endpoint }),
  });

  const faucets = await faucetClient.getAllFaucets();

  return faucets;
});

sample({
  clock: fetchFaucetsEvent,
  source: $faucetsEndpoint,
  target: fetchFaucetsFx,
});

$faucetsEndpoint.watch((endpoint) => {
  if (endpoint) {
    fetchFaucetsEvent();
  }
});

$faucets.on(fetchFaucetsFx.doneData, (_, balance) => balance);

fetchFaucetsEvent();
