import type { RouteInstance, RouteParams } from "atomic-router";
import { createDomain } from "effector";

export const searchDomain = createDomain("search");

export const createStore = searchDomain.createStore.bind(searchDomain);
export const createEvent = searchDomain.createEvent.bind(searchDomain);
export const createEffect = searchDomain.createEffect.bind(searchDomain);

export const $query = createStore<string>("");

export const $focused = createStore<boolean>(false);

export type SearchType = "block" | "message" | "address";

export type SearchItem = {
  type: SearchType;
  label: string;
  route: RouteInstance<RouteParams>;
  params: RouteParams;
};
export const $results = createStore<SearchItem[]>([]);

export const updateSearch = createEvent<string>();
export const clearSearch = createEvent();

export const focusSearch = createEvent();
export const blurSearch = createEvent();

export const unfocus = createEvent();

export const searchFx = createEffect<string, SearchItem[]>();
