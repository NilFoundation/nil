import { redirect } from "atomic-router";
import dayjs from "dayjs";
import { combine, sample } from "effector";
import { persist } from "effector-storage/local";
import { fetchSolidityCompiler } from "../../services/compiler";
import { playgroundRoute, playgroundWithHashRoute } from "../routing/routes/playgroundRoute";
import { getRuntimeConfigOrThrow } from "../runtime-config";
import {
  $code,
  $codeSnippetHash,
  $error,
  $recentProjects,
  $shareCodeSnippetError,
  $solidityVersion,
  $warnings,
  changeCode,
  changeSolidityVersion,
  compile,
  compileCodeFx,
  fetchCodeSnippetEvent,
  fetchCodeSnippetFx,
  loadedPlaygroundPage,
  setCodeSnippetEvent,
  setCodeSnippetFx,
  updateRecentProjects,
} from "./model";
import type { App } from "./types";

$code.on(changeCode, (_, x) => {
  return x;
});

persist({
  key: "code",
  store: $code,
});

compileCodeFx.use(async ({ version, code }) => {
  const compiler = await fetchSolidityCompiler(
    `https://binaries.soliditylang.org/bin/soljson-${version}.js`,
  );
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
        sourcecode: code,
        abi: contract.abi,
      });
    }
  }

  const errors = res.errors || [];
  const severeErrors = errors.filter((error) => error.severity === "error");

  if (severeErrors.length > 0) {
    throw new Error(severeErrors.map((error) => error.formattedMessage).join("\n"));
  }

  const warnings = errors.filter((error) => error.severity === "warning");
  const refinedWarnings = warnings.map((warning) => {
    const warningLines = warning.formattedMessage.split("\n");
    const locationLine = warningLines.find((line) => line.includes("-->"))?.trim();
    const [_, lineNumber] = locationLine ? locationLine.split(":") : [0, 0];

    return {
      message: warning.formattedMessage,
      line: Number(lineNumber),
    };
  });

  return { apps: contracts, warnings: refinedWarnings };
});

$solidityVersion.on(changeSolidityVersion, (_, version) => version);

persist({
  store: $solidityVersion,
  key: "solidityVersion",
});

sample({
  source: combine($code, $solidityVersion, (code, version) => ({
    code,
    version,
  })),
  clock: compile,
  target: compileCodeFx,
});

sample({
  source: combine($code, $solidityVersion, (code, version) => ({
    code,
    version,
  })),
  clock: loadedPlaygroundPage,
  target: compileCodeFx,
});

$error.reset(changeCode);
$warnings.reset(changeCode);

interface SolidityError {
  type: string; // 'error' or 'warning'
  line: number; // line number where the error occurred
  message: string; // error message
}

$error.on(compileCodeFx.failData, (_, error) => {
  function parseSolidityError(errorString: string): SolidityError[] {
    const errors: SolidityError[] = [];
    const errorLines = errorString.split("\n");

    for (let i = 0; i < errorLines.length; i++) {
      const line = errorLines[i].trim();

      if (
        line.startsWith("ParserError") ||
        line.startsWith("TypeError") ||
        line.startsWith("DeclarationError") ||
        line.startsWith("CompilerError")
      ) {
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

$warnings.on(compileCodeFx.doneData, (_, { warnings }) => warnings);

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

$codeSnippetHash.on(setCodeSnippetEvent, () => null);

sample({
  target: $codeSnippetHash,
  source: setCodeSnippetFx.doneData,
});

$shareCodeSnippetError.on(setCodeSnippetFx.fail, () => true);
$shareCodeSnippetError.reset(setCodeSnippetFx.doneData);

sample({
  clock: playgroundWithHashRoute.navigated,
  source: playgroundWithHashRoute.$params,
  fn: (params) => params.snippetHash,
  filter: (hash) => !!hash,
  target: fetchCodeSnippetFx,
});

sample({
  clock: fetchCodeSnippetFx.doneData,
  target: changeCode,
});

$codeSnippetHash.on(fetchCodeSnippetFx.doneData, () => null);

redirect({
  clock: fetchCodeSnippetFx.doneData,
  route: playgroundRoute,
  params: {},
});

sample({
  clock: fetchCodeSnippetEvent,
  source: fetchCodeSnippetEvent,
  target: fetchCodeSnippetFx,
});

persist({
  key: "recentProjects",
  store: $recentProjects,
});

sample({
  clock: updateRecentProjects,
  source: combine($code, $recentProjects, (code, projects) => ({
    code,
    projects,
  })),
  filter: ({ code }) => code.trim().length > 0,
  target: $recentProjects,
  fn: ({ code, projects }) => {
    const limit = Number(getRuntimeConfigOrThrow().RECENT_PROJECTS_STORAGE_LIMIT) || 5;
    const key = dayjs().format("YYYY-MM-DD HH:mm:ss");
    const project = { [key]: code };

    if (Object.keys(projects).length >= limit) {
      const newProjects = { ...projects };
      delete newProjects[Object.keys(projects)[0]];
      return {
        ...newProjects,
        ...project,
      };
    }

    return {
      ...projects,
      ...project,
    };
  },
});

$error.reset(compileCodeFx.doneData);
