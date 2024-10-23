import {
  type RouteInstance,
  type RouteQuery,
  createRoute as originCreateRoute,
} from "atomic-router";
import { type Event, merge, sample } from "effector";

export type ExtendedRoute<
  T extends Record<string, string | undefined> = Record<string, string | undefined>,
> = RouteInstance<T> & {
  navigated: Event<{ params: T; query: RouteQuery }>;
  paramsApplied: Event<T>;
};

export function createRoute<
  Params extends Record<string, string | undefined> = Record<string, string | undefined>,
>(): ExtendedRoute<Params> {
  const route = originCreateRoute<Params>();
  return {
    ...route,
    navigated: merge([route.opened, route.updated]),
    paramsApplied: sample({
      source: route.$params,
      filter: route.$isOpened,
      clock: [route.$isOpened, route.$params],
    }),
  };
}
