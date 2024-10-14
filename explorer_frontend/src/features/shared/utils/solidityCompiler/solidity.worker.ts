declare global {
  interface Worker {
    // biome-ignore lint/suspicious/noExplicitAny: <explanation>
    Module: any;
  }
}
declare function importScripts(...urls: string[]): void;
function browserSolidityCompiler() {
  // biome-ignore lint/suspicious/noExplicitAny: <explanation>
  const ctx: Worker = self as any;
  ctx.addEventListener("message", ({ data }) => {
    if (data.version) {
      importScripts(data.version);
      postMessage({
        installVersion: data.version,
      });
    }
    if (data.input) {
      const soljson = ctx.Module;
      if ("_solidity_compile" in soljson) {
        const compile = soljson.cwrap("solidity_compile", "string", ["string", "number"]);
        const output = JSON.parse(compile(data.input));
        postMessage(output);
      }
    }
  });
}

if (window !== self) {
  browserSolidityCompiler();
}

export { browserSolidityCompiler };
