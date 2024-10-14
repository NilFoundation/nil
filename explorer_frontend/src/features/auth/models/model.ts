import { createDomain } from "effector";
export const domain = createDomain("auth");

const createStore = domain.createStore.bind(domain);

export const $token = createStore<string | null>(null);
