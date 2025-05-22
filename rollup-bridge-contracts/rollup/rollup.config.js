import { nodeResolve } from "@rollup/plugin-node-resolve";
import packageJson from "../package.json" with { type: "json" };
const esbuild = require("rollup-plugin-esbuild").default;
import json from "@rollup/plugin-json";
import { dts } from "rollup-plugin-dts";
import filesize from "rollup-plugin-filesize";
import { createBanner, listDependencies } from "./rollupUtils.js";

const externalizedDeps = listDependencies(packageJson);

function onwarn(warning, warn) {
  if (warning.code === 'CIRCULAR_DEPENDENCY') return;
  warn(warning);
}

const getConfig = ({ outputFile, format }) => ({
  input: "./index.ts",
  output: {
    file: outputFile,
    format,
    sourcemap: true,
    inlineDynamicImports: true,
  },
  plugins: [
    nodeResolve(),
    esbuild({
      minify: true,
      legalComments: "none",
      lineLimit: 100,
      banner: createBanner(packageJson.version, new Date().getFullYear()),
    }),
    filesize(),
    json(),
  ],
  external: (id) =>
    id === "hardhat" ||
    id.startsWith("hardhat/") ||
    id === "ws" ||
    id.startsWith("ws/") ||
    externalizedDeps.some((dep) => id === dep || id.startsWith(`${dep}/`)),
  onwarn,
});

const dtsConfig = {
  input: "./index.ts",
  output: {
    file: packageJson.types,
    format: "es",
  },
  plugins: [
    dts({
      respectExternal: false,
    }),
  ],
};

const configs = [
  getConfig({
    outputFile: packageJson.main,
    format: "cjs",
  }),
  getConfig({
    outputFile: packageJson.module,
    format: "esm",
  }),
  dtsConfig,
];

module.exports = configs;

