import { createDomain } from "effector";
import type { CometaService } from "@nilfoundation/niljs";
import { getRuntimeConfigOrThrow } from "../runtime-config";

const { COMETA_SERVICE_API_URL: customCometaEndpoint } = getRuntimeConfigOrThrow();

export const cometaDomain = createDomain("cometa");

export const $customCommetaEndpoint = cometaDomain.createStore(customCometaEndpoint ?? null);
export const $cometaService = cometaDomain.createStore<CometaService | null>(null);
export const createCometaService = cometaDomain.createEvent();
export const createCometaServiceFx = cometaDomain.createEffect<string, CometaService>();
