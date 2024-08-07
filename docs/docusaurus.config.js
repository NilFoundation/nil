// @ts-check
// `@type` JSDoc annotations allow editor autocompletion and type checking
// (when paired with `@ts-check`).
// There are various equivalent ways to declare your Docusaurus config.
// See: https://docusaurus.io/docs/api/docusaurus-config

import { themes as prismThemes } from 'prism-react-renderer';

import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import remarkCodeSnippets from 'remark-code-snippets';

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: '=nil; Foundation Documentation Portal',
  tagline: '',
  favicon: 'img/nil-logo.png',

  // Set the production url of your site here
  url: 'https://docs.nil.foundation/',
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: '/',

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: '=nil; Foundation', // Usually your GitHub org/user name.
  projectName: 'docs.nil.foundation', // Usually your repo name.

  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

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
          customCss: require.resolve("./src/css/custom.css"),
        },
      }),
    ]
  ],
  markdown: {
    mermaid: true,
  },
  themes: ['@docusaurus/theme-mermaid'],
  plugins: [
    'docusaurus-plugin-goatcounter'
    ,
    [
      'docusaurus-plugin-typedoc',
      {
        out: "./nil/reference/client",
        outputFileStrategy: "members",
        fileExtension: ".mdx",
        useCodeBlocks: true,
        parametersFormat: "htmlTable",
        entryPoints: [
          process.env.NILJS_SRC
        ],
        skipErrorChecking: true,
        sidebar: {
          "autoConfiguration": true,
          "pretty": false
        },
        readme: "none",
        indexFormat: "Table",
        sanitizeComments: "true",
        excludePrivate: "true",
        exclude: [
          "**/*refiners.ts*",
          "**/*assert.ts*",
        ],
      }

    ]
  ],
  stylesheets: [
    {
      href: 'https://cdn.jsdelivr.net/npm/katex@0.13.24/dist/katex.min.css',
      type: 'text/css',
      integrity:
        'sha384-odtC+0UGzzFL/6PNoE8rX/SPcQDXBJ+uRepguP4QkPCm2LBxH3FA3y+fKSiJ+AmM',
      crossorigin: 'anonymous',
    },
  ],
  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    (
      {

        algolia: {
          appId: 'KDQGY81FVN',
          apiKey: '8ad8d801f2775ff3cf8c4433dfe290e4',
          indexName: 'nil',
          contextualSearch: true,
          searchParameters: {},
          searchPagePath: 'search',
          insights: false,
        },
        goatcounter: {
          code: 'docs-nil-foundation',
        },
        colorMode: {
          defaultMode: 'dark',
          disableSwitch: false,

        },
        navbar: {
          title: '=nil; Foundation',
          logo: {
            src: 'img/nil-logo.png',
          },
          items: [
            {
              label: 'Documentation',
              position: 'left',
              to: '/nil/intro'
            },
            {
              type: 'dropdown',
              label: 'Developer tools',
              position: 'right',
              items: [
                {
                  label: 'Block explorer',
                  href: 'https://explore.nil.foundation/'
                },
                {
                  label: 'Solidity extension',
                  href: 'https://github.com/NilFoundation/nil_cli/blob/master/Nil.sol'
                },
                {
                  label: '=nil; CLI',
                  href: 'https://github.com/NilFoundation/nil_cli/tree/master'
                },
                {
                  label: 'Client library',
                  href: 'https://www.npmjs.com/package/@nilfoundation/niljs'
                },
                {
                  label: 'Hardhat plugin',
                  href: 'https://github.com/NilFoundation/nil-hardhat-example'
                }
              ]
            }
          ],
        },
        footer: {
          links: [
            {
              title: 'Community',
              items: [
                {
                  label: 'Discord',
                  to: 'https://discord.gg/KmTAEjbmM3'
                },
                {
                  label: 'GitHub',
                  to: 'https://github.com/nilfoundation'
                },
                {
                  label: 'Telegram',
                  to: 'https://t.me/nilfoundation'
                }
              ],
            }
          ],
          style: 'dark',
          copyright: `Copyright © ${new Date().getFullYear()} =nil; Foundation`,
        },
        prism: {
          defaultLanguage: 'clike',
          theme: prismThemes.vsDark,
          darkTheme: prismThemes.vsDark,
          additionalLanguages: ['bash', 'solidity']
        },
      }),
};

export default config;
