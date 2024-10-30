import tseslint from "typescript-eslint";
import jsdoc from "eslint-plugin-jsdoc";

// eslint config for checking jsDoc types validity
// biome-ignore lint/style/noDefaultExport: <explanation>
export default [
  {
    files: ["src/clients/**/*.ts"],
    plugins: {
      jsdoc,
      "@typescript-eslint": tseslint.plugin,
    },
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {},
  },
];
