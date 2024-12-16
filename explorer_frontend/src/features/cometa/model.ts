import { createDomain } from "effector";
import type { CometaService } from "@nilfoundation/niljs";
import { getRuntimeConfigOrThrow } from "../runtime-config";

const { COMETA_SERVICE_API_URL, RPC_API_URL } = getRuntimeConfigOrThrow();

export const cometaDomain = createDomain("cometa");

export const $commetaEndpoint = cometaDomain.createStore(
  COMETA_SERVICE_API_URL || RPC_API_URL || null,
);
export const $cometaService = cometaDomain.createStore<CometaService | null>(null);
export const createCometaService = cometaDomain.createEvent();
export const createCometaServiceFx = cometaDomain.createEffect<string, CometaService>();
