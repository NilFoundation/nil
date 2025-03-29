import { createDomain } from "effector";
import type { App } from "../../code/types";

export const codeDomain = createDomain("contracts-explort-app");

export const exportApp = codeDomain.createEvent();
export const exportAppFx = codeDomain.createEffect<App, void>();
