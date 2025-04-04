import type * as esbuild from "esbuild";
import type { App } from "../../features/code/types";

export function contractResolverPlugin(contracts: App[]): esbuild.Plugin {
  return {
    name: "contract-injector",
    setup(build: esbuild.PluginBuild) {
      build.onResolve({ filter: /\.contract/ }, (args) => {
        return { path: args.path, namespace: "userContract" };
      });

      build.onLoad({ filter: /.*/, namespace: "userContract" }, async (args) => {
        try {
          const contractName = args.path.split("/").pop()?.slice(0, -9) || "";
          const app = contracts.find((app) => app.name === contractName);
          const contents = JSON.stringify({
            abi: app?.abi || "",
            bytecode: app?.bytecode || "",
          });
          const loader = "json";

          return {
            contents,
            loader,
          };
        } catch (e) {
          return {
            errors: [
              {
                text: `Error loading ${args.path}: ${e.message}`,
                location: { file: args.path },
              },
            ],
          };
        }
      });
    },
  };
}
