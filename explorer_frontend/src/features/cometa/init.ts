import { CometaService, HttpTransport } from "@nilfoundation/niljs";
import { $endpoint } from "../account-connector/model";
import {
  $cometaService,
  $customCommetaEndpoint,
  createCometaService,
  createCometaServiceFx,
} from "./model";
import { combine, sample } from "effector";

const $refinedEndpoint = combine(
  $endpoint,
  $customCommetaEndpoint,
  (endpoint, customEndpoint) => customEndpoint || endpoint,
);

$refinedEndpoint.watch((endpoint) => {
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
  source: $refinedEndpoint,
  target: createCometaServiceFx,
});

createCometaService();
