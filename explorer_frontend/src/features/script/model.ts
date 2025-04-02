import { createDomain } from "effector";
import { fetchScript, setScript } from "../../api/script";


export const scriptDomain = createDomain("script");

export const $script = scriptDomain.createStore<string>("");
export const changeScript = scriptDomain.createEvent<string>();

export const runCode = scriptDomain.createEvent();

export const $scriptError = scriptDomain.createStore<{
  message: string;
  line: number;
}[]>([]);

export const $scriptWarnings = scriptDomain.createStore<{
  message: string;
  line: number;
}[]>([]);

export const runCodeFx = scriptDomain.createEffect<
  {
    code: string;
  },
  {
    script: string;
    warnings: {
      message: string;
      line: number;
    }[];
  }
>();

export const $scriptHash = scriptDomain.createStore<string | null>(null);
export const $shareScriptError = scriptDomain.createStore<boolean>(false);

export const setScriptEvent = scriptDomain.createEvent();
export const fetchScriptEvent = scriptDomain.createEvent<string>();

export const setScriptFx = scriptDomain.createEffect<string, string>();
export const fetchScriptFx = scriptDomain.createEffect<string, string>();

setScriptFx.use((script) => {
  return setScript(script);
});

fetchScriptFx.use((hash) => {
  return fetchScript(hash);
});

export const loadedScriptPage = scriptDomain.createEvent();

export const $recentScripts = scriptDomain.createStore<Record<string, string>>({});

