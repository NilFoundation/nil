import { createDomain } from "effector";
import { fetchShards } from "../../../api/shards";

type Shards = Awaited<ReturnType<typeof fetchShards>>;

export const explorerShardsList = createDomain("shards-list");

const createStore = explorerShardsList.createStore.bind(explorerShardsList);
const createEffect = explorerShardsList.createEffect.bind(explorerShardsList);

export const $shards = createStore<Shards>([]);

export const fetchShardsFx = createEffect<void, Shards, Error>();

fetchShardsFx.use(fetchShards);
