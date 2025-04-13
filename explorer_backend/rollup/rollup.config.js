import { nodeResolve } from "@rollup/plugin-node-resolve";
import packageJson from "../package.json" with { type: "json" };
const esbuild = require("rollup-plugin-esbuild").default;
import json from "@rollup/plugin-json";
import filesize from "rollup-plugin-filesize";
import commonjs from "@rollup/plugin-commonjs";
import copy from "rollup-plugin-copy";

const getConfig = ({ outputFile, format }) => ({
  input: "index.ts",
  output: {
    file: outputFile,
    format: "cjs",
    sourcemap: true,
    inlineDynamicImports: true,
  },
  plugins: [
    nodeResolve(

    ),
    commonjs({
      include: /node_modules/,
      requireReturnsDefault: "auto",
    }),
    esbuild({
      minify: true,
      legalComments: "none",
      lineLimit: 100,
    }),
    filesize(),
    json(),
    copy({
      targets: [{ src: "node_modules/node-sqlite3-wasm/dist/node-sqlite3-wasm.wasm", dest: "dist/" }],
      verbose: true,
    }),
  ],
  external:  [],
});

const configs = [
  getConfig({
    outputFile: packageJson.main,
    format: "cjs",
  }),
];

module.exports = configs;
