import { CometaService, HttpTransport } from "@nilfoundation/niljs";
import { $endpoint } from "../account-connector/model";
import { $cometaService, createCometaService, createCometaServiceFx } from "./model";
import { sample } from "effector";

$endpoint.watch((endpoint) => {
  if (endpoint) {
    createCometaService();
  }
});

createCometaServiceFx.use(async (endpoint) => {
  const cometaService = new CometaService({
    transport: new HttpTransport({ endpoint }),
  });

  return cometaService;
});

$cometaService.on(createCometaServiceFx.doneData, (_, cometaService) => cometaService);

sample({
  clock: createCometaService,
  source: $endpoint,
  target: createCometaServiceFx,
});

createCometaService();
