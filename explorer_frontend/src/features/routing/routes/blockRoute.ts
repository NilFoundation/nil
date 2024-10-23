import type { RouteInstance } from "atomic-router";
import { createRoute } from "../utils/createRoute";

export const blockRoute = createRoute<{ shard: string; id: string }>() satisfies RouteInstance<{
  shard: string;
  id: string;
}>;

export const blockDetailsRoute = createRoute<{
  shard: string;
  id: string;
  details: string;
}>() satisfies RouteInstance<{
  shard: string;
  id: string;
  details: string;
}>;
