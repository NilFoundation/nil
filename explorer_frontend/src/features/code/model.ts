import { createDomain, createEvent } from "effector";
import type { App } from "../../types";
import { fetchCodeSnippet, setCodeSnippet } from "../../api/code";

export const codeDomain = createDomain("code");

const createStore = codeDomain.createStore.bind(codeDomain);
const createEffect = codeDomain.createEffect.bind(codeDomain);

export const $code = codeDomain.createStore<string>("");
export const changeCode = codeDomain.createEvent<string>();
export const compile = codeDomain.createEvent();
export const $solidityVersion = codeDomain.createStore("soljson-v0.8.26+commit.8a97fa7a.js");
export const changeSolidityVersion = codeDomain.createEvent<string>();
export const $error = codeDomain.createStore<
  {
    message: string;
    line: number;
  }[]
>([]);
export const compileCodeFx = codeDomain.createEffect<
  {
    code: string;
    version: string;
  },
  App[]
>();

export const $codeSnippetHash = codeDomain.createStore<string | null>(null);
export const $shareCodeSnippetError = codeDomain.createStore<boolean>(false);

export const setCodeSnippetEvent = codeDomain.createEvent();
export const fetchCodeSnippetEvent = codeDomain.createEvent<string>();

export const setCodeSnippetFx = codeDomain.createEffect<string, string>();
export const fetchCodeSnippetFx = codeDomain.createEffect<string, string>();

setCodeSnippetFx.use((code) => {
  return setCodeSnippet(code);
});

fetchCodeSnippetFx.use((hash) => {
  return fetchCodeSnippet(hash);
});

export const loadedPage = createEvent();
