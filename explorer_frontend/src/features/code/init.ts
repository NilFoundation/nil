import { redirect } from "atomic-router";
import dayjs from "dayjs";
import { combine, sample } from "effector";
import { persist } from "effector-storage/local";
import { fetchSolidityCompiler } from "../../services/compiler";
import { playgroundRoute, playgroundWithHashRoute } from "../routing/routes/playgroundRoute";
import { getRuntimeConfigOrThrow } from "../runtime-config";
import {
  $code,
  $codeError,
  $projectHash,
  $codeWarnings,
  $recentProjects,
  $script,
  $scriptErrors,
  $scriptWarnings,
  $shareProjectError,
  $solidityVersion,
  changeCode,
  changeScript,
  changeSolidityVersion,
  compileCode,
  compileCodeFx,
  fetchProjectEvent,
  fetchProjectFx,
  loadedPlaygroundPage,
  runScriptFx,
  setProjectEvent,
  setProjectFx,
  updateRecentProjects,
  runScript,
} from "./model";
import type { App } from "./types";
import { $contracts } from "../contracts/models/base";
import { bundle } from "../../services/bundler"
import { run } from "../../services/runner";

const consoleLogRegex = /console\.log\s*\(\s*(?:"(?:\\.|[^"\\])*"|`(?:\\.|[^`\\])*`)\s*\)/g;
const consoleWarnRegex = /console\.warn\s*\(\s*(?:"(?:\\.|[^"\\])*"|`(?:\\.|[^`\\])*`)\s*\)/g;

$code.on(changeCode, (_, x) => {
  return x;
});

$script.on(changeScript, (_, x) => {
  return x ?? "";
});

persist({
  key: "code",
  store: $code,
});

persist({
  key: "script",
  store: $script,
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

sample({
  clock: compileCodeFx.doneData,
  source: combine($contracts, $script),
  fn: ([contracts, script]) => {
    const cleanedScript = script.replace(
      /^\s*import\s+(?:[^'"]+from\s+)?['"][^'"]+\.contract['"]\s*;?\s*$/gm,
      ''
    );

    let res = cleanedScript;

    for (const app of contracts) {
      res = `import ${app.name} from "./contracts/${app.name}.contract";\r\n${res}`;
    }

    return res;
  },
  target: changeScript
});



sample({
  clock: runScript,
  source: combine($script, $contracts),
  fn: ([script, contracts]) => {
    return {
      script,
      contracts,
    };
  },
  target: runScriptFx,
});

runScriptFx.use(async ({ script, contracts }) => {
  console.log("Running script:", script);
  const res = await bundle(script, contracts);
  console.log("Bundled script:", res);
  const consoleLogs = [...script.matchAll(consoleLogRegex)].map(match => match[1] || match[2]);
  const consoleWarns = [...script.matchAll(consoleWarnRegex)].map(match => match[1] || match[2]);
  run(res);

  return {
    script: res,
    warnings: [],
  };
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
  clock: compileCode,
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

$codeError.reset(changeCode);
$codeWarnings.reset(changeCode);

$scriptErrors.reset(changeScript);
$scriptWarnings.reset(changeScript);

interface SolidityError {
  type: string; // 'error' or 'warning'
  line: number; // line number where the error occurred
  message: string; // error message
}

$codeError.on(compileCodeFx.failData, (_, error) => {
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

$codeWarnings.on(compileCodeFx.doneData, (_, { warnings }) => warnings);

$scriptWarnings.on(runScriptFx.doneData, (_, { warnings }) => warnings);


sample({
  clock: setProjectEvent,
  source: combine($code, $script),
  fn: ([code, script]) => ({
    code,
    script,
  }),
  target: setProjectFx,
});

sample({
  clock: setProjectEvent,
  source: combine($code, $script),
  target: $shareProjectError,
  fn: () => false,
});

$projectHash.on(setProjectEvent, () => null);

sample({
  target: $projectHash,
  source: setProjectFx.doneData,
});

$shareProjectError.on(setProjectFx.fail, () => true);
$shareProjectError.reset(setProjectFx.doneData);

sample({
  clock: playgroundWithHashRoute.navigated,
  source: playgroundWithHashRoute.$params,
  fn: (params) => params.snippetHash,
  filter: (hash) => !!hash,
  target: fetchProjectFx,
});

sample({
  clock: fetchProjectFx.doneData,
  fn: ({ code }) => code,
  target: changeCode,
});

sample({
  clock: fetchProjectFx.doneData,
  fn: ({ script }) => script,
  target: changeScript,
});

$projectHash.on(fetchProjectFx.doneData, () => null);

redirect({
  clock: fetchProjectFx.doneData,
  route: playgroundRoute,
  params: {},
});

sample({
  clock: fetchProjectEvent,
  source: fetchProjectEvent,
  target: fetchProjectFx,
});

persist({
  key: "recentProjects",
  store: $recentProjects,
});

sample({
  clock: updateRecentProjects,
  source: combine($code, $script, $recentProjects, (code, script, projects) => ({
    code,
    script,
    projects,
  })),
  filter: ({ code }) => code.trim().length > 0,
  target: $recentProjects,
  fn: ({ code, script, projects }) => {
    const limit = Number(getRuntimeConfigOrThrow().RECENT_PROJECTS_STORAGE_LIMIT) || 5;
    const key = dayjs().format("YYYY-MM-DD HH:mm:ss");
    const value = [code, script].join("\r\n");
    const project = { [key]: value };

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

$codeError.reset(compileCodeFx.doneData);
