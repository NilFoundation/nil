import { nodeResolve } from "@rollup/plugin-node-resolve";
import packageJson from "../package.json" with { type: "json" };
import esbuild from "rollup-plugin-esbuild";
import json from "@rollup/plugin-json";
import { dts } from "rollup-plugin-dts";
import filesize from "rollup-plugin-filesize";
import del from "rollup-plugin-delete";

const createBanner = (version, year) => {
  return `/**!
 * @nilfoundation/hardhat-plugin v${version}
 *
 * @copyright (c) ${year} =nil; Foundation.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE.md file in the root directory of this source tree.
 *
 * @license MIT
 */`.trim();
};

const getConfig = ({ outputFile, format, deleteDist }) => ({
  input: "src/index.ts",
  output: {
    file: outputFile,
    format,
    sourcemap: true,
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
    deleteDist
      ? del({
          targets: ["dist"],
        })
      : null,
  ],
  external: (id) => {
    if (id.includes("node_modules")) {
      return true;
    }
    if (id.startsWith("@nilfoundation/")) {
      return true;
    }
    return false;
  },
});

const dtsConfig = {
  input: "src/index.ts",
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
    deleteDist: true,
  }),
  getConfig({
    outputFile: packageJson.module,
    format: "esm",
  }),
  dtsConfig,
];

export default configs;
