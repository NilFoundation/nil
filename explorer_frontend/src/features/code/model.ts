import { createDomain, createEvent } from "effector";
import type { App } from "../../types";
import { fetchCodeSnippet, setCodeSnippet } from "../../api/code";

export const codeDomain = createDomain("code");

const createStore = codeDomain.createStore.bind(codeDomain);
const createEffect = codeDomain.createEffect.bind(codeDomain);

export const $code = createStore<string>("");
export const changeCode = createEvent<string>();
export const compile = createEvent();
export const $solidityVersion = createStore("soljson-v0.8.26+commit.8a97fa7a.js");
export const changeSolidityVersion = createEvent<string>();
export const $error = createStore<
  {
    message: string;
    line: number;
  }[]
>([]);
export const compileCodeFx = createEffect<
  {
    code: string;
    version: string;
  },
  App[]
>();

export const $codeSnippetHash = createStore<string | null>(null);
export const $shareCodeSnippetError = createStore<boolean>(false);

export const setCodeSnippetEvent = createEvent();
export const fetchCodeSnippetEvent = createEvent();

export const setCodeSnippetFx = createEffect<string, string>();
export const fetchCodeSnippetFx = createEffect<string, string>();

setCodeSnippetFx.use((code) => {
  return setCodeSnippet(code);
});

fetchCodeSnippetFx.use((hash) => {
  return fetchCodeSnippet(hash);
});

export const loadedPage = createEvent();
