#!/usr/bin/env node

import fs from 'node:fs/promises';
import path from 'path';
import util from 'node:util';
import { exec as execCallback } from 'node:child_process';
import { fileURLToPath } from 'url';
import { NIL_GLOBAL } from '../../tests/globals';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);



const exec = util.promisify(execCallback);

interface CommandSpec {
  name: string;
  description: string | null;
  usage: string | null;
  regularFlags: Flag[];
  globalFlags: Flag[];
}

interface Flag {
  short?: string;
  name: string;
  type: string;
  description: string;
}

async function prettifyCommandNames(key: string, commandNamesArray: string[]): Promise<string[]> {

  let result: string[] = [];
  for (const name of commandNamesArray) {
    const formattedName = name.replace('.go', '');
    switch (formattedName) {
      case 'command':
        result.push(`${NIL_GLOBAL} ${key} -h`);
        break;
      case `${key}`:
        result.push(`${NIL_GLOBAL} ${key} -h`);
        break;
      default:
        result.push(`${NIL_GLOBAL} ${key} ${formattedName} -h`);
        break;
    }
  }
  return result;
}

async function generateCommandNames(commandsPath: string): Promise<Record<string, string[]>> {
  let result: Record<string, string[]> = {};
  const coreCommands = await fs.readdir(commandsPath, {});

  for (const coreCommand of coreCommands) {
    const fullCommandPath = path.resolve(commandsPath, coreCommand);
    const commands = await fs.readdir(fullCommandPath, {});

    for (const command of commands) {
      const fullSubCommandPath = path.resolve(fullCommandPath, command);
      if (!command.endsWith('params.go') && command.endsWith('.go')) {
        const commandFileContents = await fs.readFile(fullSubCommandPath, 'utf8');

        const useRegex = /Use:\s*["']([\w-]+)(?:\s|\[|["'])/g;
        const matchesUse = Array.from(commandFileContents.matchAll(useRegex));

        const extractedUseCommands = matchesUse.map(match => match[1]);

        const methodRegex = /method\s*(?:=|:=)\s*["']([^"']+)["']/gi;
        const minterMatches = Array.from(commandFileContents.matchAll(methodRegex));

        const extractedMinterCommands = minterMatches.map(match => {
          const method = match[1];
          return `${method.toLowerCase()}-currency`;
        });

        let extractedCommands = extractedUseCommands.concat(extractedMinterCommands);

        if (extractedCommands.length > 0) {
          if (!result[coreCommand]) {
            result[coreCommand] = extractedCommands;
          } else {
            result[coreCommand] = result[coreCommand].concat(extractedCommands);
          }
        }
      }
    }
  }

  const prettifiedResult: Record<string, string[]> = {};
  for (const key of Object.keys(result)) {
    prettifiedResult[key] = await prettifyCommandNames(key, result[key]);
  }

  return prettifiedResult;
}

async function generateCommandSpec(commandName: string): Promise<CommandSpec | null> {
  try {
    const { stdout, stderr } = await exec(commandName);
    if (stderr) {
      console.error(`Error executing "${commandName}":`, stderr);
      return null;
    }

    const descriptionMatch = stdout.match(/^(.*?)\s*Usage:/);
    const commandDescription = descriptionMatch ? descriptionMatch[1].trim() : null;

    const usageMatch = stdout.match(/Usage:\s*([^\n]+)/);
    const usage = usageMatch ? usageMatch[1].trim() : null;

    let usageArgsEnhanced: string[] = [];
    if (usageMatch) {
      const usageLine = usageMatch[1];
      const bracketRegex = /\[([^\]]+)\]/g;
      let match: RegExpExecArray | null;
      while ((match = bracketRegex.exec(usageLine)) !== null) {
        const arg = match[1].trim();
        if (arg.toLowerCase() !== 'flags') {
          usageArgsEnhanced.push(arg);
        }
      }
    }

    const flagsSectionMatch = stdout.match(/Flags:\s*\n([\s\S]*?)(?=\n\S|\n$)/);
    const flagsSection = flagsSectionMatch ? flagsSectionMatch[1] : '';

    const globalFlagsSectionMatch = stdout.match(/Global Flags:\s*\n([\s\S]*?)(?=\n\S|\n$)/);
    const globalFlagsSection = globalFlagsSectionMatch ? globalFlagsSectionMatch[1] : '';

    const regularFlags = parseFlags(flagsSection);
    const globalFlags = parseFlags(globalFlagsSection);

    return {
      name: commandName.replace('-h', '').trim(),
      description: commandDescription,
      usage: usage,
      regularFlags: regularFlags,
      globalFlags: globalFlags,
    };
  } catch (error) {
    console.error(`Failed to execute command "${commandName}":`, error);
    return null;
  }
}

function parseFlags(text: string): Flag[] {
  const lines = text.split('\n').filter(line => line.trim());
  const flags: Flag[] = [];
  const KNOWN_TYPES = new Set([
    'string',
    'boolean',
    'int',
    'float',
    'duration',
    'file',
    'stringarray',
    'value',
    'shardid',
  ]);

  const shortAndLongFlagRegex = /^(-\w),\s+(--[\w-]+)(?:\s+([\w\[\]\/]+))?\s+(.*)$/;
  const longFlagRegex = /^(--[\w-]+)(?:\s+([\w\[\]\/]+))?\s+(.*)$/;

  lines.forEach(line => {
    line = line.trim();
    let match = line.match(shortAndLongFlagRegex);
    if (match) {
      let [, short, name, type, description] = match;

      if (type && !KNOWN_TYPES.has(type.toLowerCase())) {
        description = `${type} ${description}`;
        type = 'boolean';
      } else {
        type = type || 'boolean';
      }

      flags.push({
        short,
        name,
        type: type.toLowerCase(),
        description: description.trim().replace('khannanov-nil', 'usr'),
      });
      return;
    }

    match = line.match(longFlagRegex);
    if (match) {
      let [, name, type, description] = match;

      if (type && !KNOWN_TYPES.has(type.toLowerCase())) {
        description = `${type} ${description}`;
        type = 'boolean';
      } else {
        type = type || 'boolean';
      }

      flags.push({
        name,
        type: type.toLowerCase(),
        description: description.trim(),
      });
    }
  });

  return flags;
}

function prettifyFlagContent(flags: Flag[]): string {
  let result = '| Name | Type | Description |\r\n|:--:|:--:|--|\r\n';
  for (const flag of flags) {
    const short = flag.short ? `\`${flag.short}\`, ` : '';
    result += `| ${short}\`${flag.name}\` | \`${flag.type}\` | ${flag.description} |\r\n`;
  }
  result += '\r\n';
  return result.replace(/[<>]/g, '');
}

async function fillCommandFileContents(commandSpec: CommandSpec, filePath: string): Promise<void> {
  const fileContents = `# \`${commandSpec.name}\`\r\n\r\n## Description\r\n\r\n${commandSpec.description}\r\n\r\n## Usage\r\n\r\n\`${commandSpec.usage}\`\r\n\r\n## Regular Flags\r\n\r\n${prettifyFlagContent(commandSpec.regularFlags)}\r\n\r\n## Global Flags\r\n\r\n${prettifyFlagContent(commandSpec.globalFlags)}\r\n`;
  await fs.writeFile(filePath, fileContents, 'utf8');
}

async function createHighLevelDirs(commandSpecs: Record<string, CommandSpec>, outputDir: string) {
  for (const key of Object.keys(commandSpecs)) {
    const commandName = key.split(' ')[1];
    const newDirPath = path.resolve(outputDir, commandName);
    await fs.mkdir(newDirPath, { recursive: true });
  }
}

async function createCommandFiles(commandSpecs: Record<string, CommandSpec>, outputDir: string) {
  for (const key of Object.keys(commandSpecs)) {

    let filePath;
    const dirName = path.resolve(outputDir, key.split(' ')[1]);
    const dirPart = dirName.split('/').at(-1);
    const fileName = key.split(' ').at(-2);
    if (dirPart == fileName) {
      filePath = path.resolve(dirName, 'index.mdx');
    } else {
      filePath = path.resolve(dirName, `${fileName}.mdx`);

    }

    await fillCommandFileContents(commandSpecs[key], filePath);
  }
}

async function generateCommandSpecs(commandsPath: string, outputDir: string): Promise<Record<string, CommandSpec>> {
  let result: Record<string, CommandSpec> = {};

  console.log('Generating command names...');
  const commandNames = await generateCommandNames(commandsPath);

  console.log('Generating command specifications...');
  const allCommands = Object.values(commandNames).flat();

  for (const command of allCommands) {
    const commandSpec = await generateCommandSpec(command);
    if (commandSpec) {
      result[command] = commandSpec;
    }
  }

  console.log('Creating directories...');
  await createHighLevelDirs(result, outputDir);

  console.log('Creating command files...');
  await createCommandFiles(result, outputDir);

  return result;
}

async function main() {
  const commandsPath = path.resolve(process.env.CMD_NIL!);
  const outputDir = path.resolve(__dirname, '../reference/cli-reference');

  try {
    await generateCommandSpecs(commandsPath, outputDir);
    console.log('CLI Reference generation completed successfully.');
  } catch (error) {
    console.error('An error occurred during CLI Reference generation:', error);
    process.exit(1);
  }
}

main();
