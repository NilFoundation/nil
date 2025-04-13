// @ts-check
// `@type` JSDoc annotations allow editor autocompletion and type checking
// (when paired with `@ts-check`).
// There are various equivalent ways to declare your Docusaurus config.
// See: https://docusaurus.io/docs/api/docusaurus-config

import { themes as prismThemes } from "prism-react-renderer";

import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import remarkCodeSnippets from "nil-remark-code-snippets";

import path from "node:path";

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: "=nil; Foundation Documentation Portal",
  tagline: "",
  favicon: "img/nil-logo.png",

  // Set the production url of your site here
  url: "https://docs.nil.foundation/",
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: "/",

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: "=nil; Foundation", // Usually your GitHub org/user name.
  projectName: "docs.nil.foundation", // Usually your repo name.

  onBrokenLinks: "log",
  onBrokenMarkdownLinks: "warn",
  onBrokenAnchors: "log",

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  scripts: [
    {
      src: "https://docs.nil.foundation/goatcounter/count.js",
      async: true,
      "data-goatcounter": "https://docs.nil.foundation/goatcounter/count",
    },
  ],

  presets: [
    [
      "@khannanov-nil/openrpc-docusaurus/dist/preset",
      /** @type {import('@khannanov-nil/openrpc-docusaurus/dist/preset').Options} */
      ({
        docs: {
          path: "nil",
          routeBasePath: "nil",
          sidebarPath: require.resolve("./sidebar-nil.js"),
          remarkPlugins: [remarkMath, remarkCodeSnippets],
          rehypePlugins: [rehypeKatex],
          openrpc: {
            openrpcDocument: process.env.OPENRPC_JSON,
            path: "references",
            sidebarLabel: "JSON-RPC API",
          },
        },
        theme: {
          customCss: [
            require.resolve("./src/css/custom.css"),
            require.resolve("./src/css/assistant.css"),
            require.resolve("./src/css/modal.css"),
          ],
        },
      }),
    ],
  ],
  markdown: {
    mermaid: true,
  },
  themes: ["@docusaurus/theme-mermaid"],
  plugins: [
    [path.resolve(__dirname, "./plugins/custom-loader"), {}],
    [path.resolve(__dirname, "./plugins/nil-playground-plugin"), {}],
    [
      "nil-docusaurus-plugin-typedoc",
      {
        out: "./nil/reference/client",
        outputFileStrategy: "members",
        fileExtension: ".mdx",
        useCodeBlocks: true,
        parametersFormat: "htmlTable",
        entryPoints: [`${process.env.NILJS_SRC}/src/index.ts`],
        tsconfig: `${process.env.NILJS_SRC}/tsconfig.json`,
        skipErrorChecking: true,
        sidebar: {
          autoConfiguration: true,
          pretty: false,
        },
        readme: "none",
        indexFormat: "Table",
        sanitizeComments: "true",
        excludePrivate: "true",
        exclude: ["**/*refiners.ts*", "**/*assert.ts*"],
      },
    ],
  ],
  stylesheets: [
    {
      href: "https://cdn.jsdelivr.net/npm/katex@0.13.24/dist/katex.min.css",
      type: "text/css",
      integrity: "sha384-odtC+0UGzzFL/6PNoE8rX/SPcQDXBJ+uRepguP4QkPCm2LBxH3FA3y+fKSiJ+AmM",
      crossorigin: "anonymous",
    },
    {
      href: "https://fonts.googleapis.com/icon?family=Material+Icons",
    },
  ],
  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      algolia: {
        appId: "KDQGY81FVN",
        apiKey: "8ad8d801f2775ff3cf8c4433dfe290e4",
        indexName: "nil",
        contextualSearch: true,
        searchParameters: {},
        searchPagePath: "search",
        insights: false,
      },
      colorMode: {
        defaultMode: "dark",
        disableSwitch: false,
      },
      navbar: {
        title: "=nil; Foundation",
        logo: {
          src: "img/nil-logo.png",
          href: "https://nil.foundation",
        },
        items: [
          {
            label: "Docs",
            position: "left",
            to: "/nil/intro",
          },
          {
            label: "Cookbook",
            position: "left",
            to: "/nil/cookbook",
          },
          {
            label: "Migration guides",
            position: "left",
            to: "nil/migration-guides/march-1803-2025-release",
          },
          {
            label: "Videos",
            position: "left",
            to: "nil/vids/uniswap-v2",
          },
          {
            type: "dropdown",
            label: "Dev tools",
            position: "left",
            items: [
              {
                label: "Block explorer",
                href: "https://explore.nil.foundation/",
              },
              {
                label: "Playground",
                href: "https://explore.nil.foundation/playground"
              },
              {
                label: "Interactive tutorials",
                href: "https://explore.nil.foundation/tutorial/async-call"
              },
              {
                label: "Solidity extension",
                href: "https://www.npmjs.com/package/@nilfoundation/smart-contracts",
              },
              {
                label: "Client library",
                href: "https://www.npmjs.com/package/@nilfoundation/niljs",
              },
            ],
          },
        ],
      },
      footer: {
        links: [
          {
            title: "Community",
            items: [
              {
                label: "Telegram",
                to: "https://t.me/nilfoundation",
              },
              {
                label: "GitHub",
                to: "https://github.com/nilfoundation",
              },
            ],
          },
        ],
        style: "dark",
        copyright: `
          Copyright © ${new Date().getFullYear()} =nil; Foundation
          This site is protected by reCAPTCHA and Google.
          <a href="https://policies.google.com/privacy">Privacy Policy</a> and
          <a href="https://policies.google.com/terms">Terms of Service</a> apply.
        `,
      },
      prism: {
        defaultLanguage: "clike",
        theme: prismThemes.vsDark,
        darkTheme: prismThemes.vsDark,
        additionalLanguages: ["bash", "solidity"],
      },
    }),
};

export default config;
