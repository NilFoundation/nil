import { createDomain } from "effector";
import type { CometaService } from "@nilfoundation/niljs";

export const cometaDomain = createDomain("cometa");

export const $cometaService = cometaDomain.createStore<CometaService | null>(null);
export const createCometaService = cometaDomain.createEvent();
export const createCometaServiceFx = cometaDomain.createEffect<string, CometaService>();
