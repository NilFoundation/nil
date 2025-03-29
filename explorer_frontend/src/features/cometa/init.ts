import { CometaClient, HttpTransport } from "@nilfoundation/niljs";
import { combine, sample } from "effector";
import { $rpcUrl } from "../account-connector/model";
import { $cometaApiUrl, $cometaClient, createCometaService, createCometaServiceFx } from "./model";

const $refinedCometaApiUrl = combine(
  $rpcUrl,
  $cometaApiUrl,
  (rpcUrl, customCometaApiUrl) => rpcUrl || customCometaApiUrl,
);

$refinedCometaApiUrl.watch((url) => {
  if (url) {
    createCometaService();
  }
});

createCometaServiceFx.use(async (endpoint) => {
  const cometaClient = new CometaClient({
    transport: new HttpTransport({ endpoint }),
  });

  return cometaClient;
});

$cometaClient.on(createCometaServiceFx.doneData, (_, cometaClient) => cometaClient);

sample({
  clock: createCometaService,
  source: $refinedCometaApiUrl,
  target: createCometaServiceFx,
});

createCometaService();
