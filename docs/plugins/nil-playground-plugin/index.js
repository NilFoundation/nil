import fs from "node:fs/promises";
import path from "node:path";

export default async function nilPlaygroundPlugin(context, options) {
  return {
    name: "nil-playground-plugin",
    getThemePath() {
      return path.resolve(__dirname, "theme");
    },
    async loadContent() {
      const getCodes = async () => {
        const contractCodes = {};
        const tests = path.resolve(__dirname, "../../tests");
        const files = await fs.readdir(tests, {});

        for (const file of files) {
          const filePath = path.resolve(tests, file);
          if (filePath.endsWith(".sol")) {
            const data = await fs.readFile(filePath, "utf8");
            contractCodes[file] = data;
          }
        }
        return contractCodes;
      };

      const result = await getCodes();
      return result;
    },
    async contentLoaded({ content, actions }) {
      const { setGlobalData } = actions;
      setGlobalData({ contractCodes: content });
    },
  };
}
