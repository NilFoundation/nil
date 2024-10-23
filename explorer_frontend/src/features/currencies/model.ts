import { createDomain } from "effector";
import { $endpoint } from "../account-connector/models/model";
import type { Hex } from "@nilfoundation/niljs";

export const faucetsDomain = createDomain("faucet");

export const $faucetsEndpoint = $endpoint.map((endpoint) => {
  const faucetEndpointPrefix = "faucet/";
  const divider = "api/";

  const parts = endpoint.split(divider);

  const faucetEndpoint = `${parts[0]}${divider}${faucetEndpointPrefix}${parts[1]}`;

  return faucetEndpoint;
});

export const $faucets = faucetsDomain.createStore<Record<string, Hex> | null>(null);

export const fetchFaucetsEvent = faucetsDomain.createEvent();

export const fetchFaucetsFx = faucetsDomain.createEffect<string, Record<string, Hex>>();
