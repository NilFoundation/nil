import { persist } from "effector-storage/local";
import {
  $code,
  $error,
  $codeSnippetHash,
  $solidityVersion,
  changeCode,
  changeSolidityVersion,
  compile,
  compileCodeFx,
  setCodeSnippetEvent,
  setCodeSnippetFx,
  $shareCodeSnippetError,
  fetchCodeSnippetFx,
  loadedPage,
} from "./model";
import { fetchSolidityCompiler } from "../../services/compiler";
import type { App } from "../../types";
import { combine, sample } from "effector";
import { sandboxWithHashRoute } from "../routing/routes/sandboxRoute";

$code.on(changeCode, (_, x) => x);

persist({
  key: "code",
  store: $code,
});

compileCodeFx.use(async ({ version, code }) => {
  const compiler = await fetchSolidityCompiler(`https://binaries.soliditylang.org/bin/${version}`);
  const res = await compiler.compile({
    code: code,
  });
  const contracts: App[] = [];
  if ("contracts" in res && res.contracts !== undefined && "Compiled_Contracts" in res.contracts) {
    for (const name in res.contracts?.Compiled_Contracts) {
      const contract = res.contracts.Compiled_Contracts[name];
      contracts.push({
        name: name,
        bytecode: `0x${contract.evm.bytecode.object}`,
        abi: contract.abi,
      });
    }
  }

  return contracts;
});

$solidityVersion.on(changeSolidityVersion, (_, version) => version);

persist({
  store: $solidityVersion,
  key: "solidityVersion",
});

sample({
  source: combine($code, $solidityVersion, (code, version) => ({ code, version })),
  clock: compile,
  target: compileCodeFx,
});
$error.reset(changeCode);
interface SolidityError {
  type: string; // 'error' or 'warning'
  line: number; // line number where the error occurred
  message: string; // error message
}
$error.on(compileCodeFx.failData, (_, error) => {
  console.log('error', error);
  function parseSolidityError(errorString: string): SolidityError[] {
    const errors: SolidityError[] = [];
    const errorLines = errorString.split("\n");

    for (let i = 0; i < errorLines.length; i++) {
      const line = errorLines[i].trim();

      if (line.startsWith("ParserError") || line.startsWith("TypeError") || line.startsWith("DeclarationError")) {
        const [type, ...messageParts] = line.split(":");
        const message = messageParts.join(":").trim();
        const locationLine = errorLines[i + 1].trim();
        const [_, lineNumber] = locationLine.split(":");

        errors.push({
          type: type.trim(),
          line: +lineNumber,
          message: message,
        });

        i += 2; // Skip the next two lines as they have been processed
      }
    }

    return errors;
  }

  const errors = parseSolidityError(error.message);

  return errors.map((error) => {
    return {
      message: error.message,
      line: error.line,
    };
  });
});

sample({
  clock: setCodeSnippetEvent,
  source: $code,
  target: setCodeSnippetFx,
});

sample({
  clock: setCodeSnippetEvent,
  source: $code,
  target: $shareCodeSnippetError,
  fn: () => false,
});

sample({
  target: $codeSnippetHash,
  source: setCodeSnippetFx.doneData,
});

$shareCodeSnippetError.on(setCodeSnippetFx.fail, () => true);

sample({
  clock: sandboxWithHashRoute.navigated,
  source: sandboxWithHashRoute.$params,
  fn: (params) => params.snippetHash,
  filter: (hash) => !!hash,
  target: fetchCodeSnippetFx,
});

sample({
  clock: fetchCodeSnippetFx.doneData,
  target: changeCode,
});

sample({
  clock: loadedPage,
  filter: $code.map((code) => !(code.trim().length === 0)),
  target: compile,
});
