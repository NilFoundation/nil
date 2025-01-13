import { createDomain, createEvent } from "effector";
import type { App } from "../../types";
import { fetchCodeSnippet, setCodeSnippet } from "../../api/code";

export const codeDomain = createDomain("code");

export const $code = codeDomain.createStore<string>("");
export const changeCode = codeDomain.createEvent<string>();
export const compile = codeDomain.createEvent();
export const $solidityVersion = codeDomain.createStore("v0.8.26+commit.8a97fa7a");
export const $availableSolidityVersions = codeDomain.createStore([
  "v0.8.28+commit.7893614a",
  "v0.8.27+commit.40a35a09",
  "v0.8.26+commit.8a97fa7a",
  "v0.8.25+commit.b61c2a91",
  "v0.8.24+commit.e11b9ed9",
]);

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
