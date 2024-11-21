import path from "node:path";

export default async function nilGoatcounterPlugin(context, options) {
  return {
    name: "nil-goatcounter-plugin",
    getThemePath() {
      return path.resolve(__dirname, "theme");
    },
    async loadContent() {},
    async contentLoaded({ content, actions }) {},
  };
}
