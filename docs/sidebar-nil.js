export default {
  nil: [
    {
      type: "category",
      label: "Introduction",
      collapsed: false,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Overview",
          id: "intro",
        },
        {
          type: "doc",
          label: "Prerequisites",
          id: "getting-started/prerequisites",
        },
        {
          type: "doc",
          label: "=nil; 101",
          id: "getting-started/nil-101",
        },
        {
          type: "doc",
          label: "Running the cluster locally",
          id: "getting-started/nild",
        },
      ],
    },
    {
      type: "category",
      label: "Getting started",
      collapsed: false,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Creating a smart account",
          id: "getting-started/essentials/creating-a-smart-account",
        },
        {
          type: "doc",
          label: "Tokens support",
          id: "getting-started/essentials/tokens",
        },
        {
          type: "doc",
          label: "Reading and writing to =nil;",
          id: "getting-started/essentials/reading-writing-info",
        },
        {
          type: "doc",
          label: "Receiving external transactions",
          id: "getting-started/essentials/receiving-ext-transactions",
        },
        {
          type: "doc",
          label: "Handling async execution",
          id: "getting-started/essentials/handling-async-execution",
        },
        {
          type: "doc",
          label: "Gas and value",
          id: "getting-started/essentials/gas-and-value",
        },

        {
          type: "category",
          label: "Working with smart contracts",
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: "doc",
              label: "Writing a smart contract",
              id: "getting-started/working-with-smart-contracts/writing-a-contract",
            },
            {
              type: "doc",
              label: "Deploying a smart contract",
              id: "getting-started/working-with-smart-contracts/deploying-a-contract",
            },
            {
              type: "doc",
              label: "Calling methods inside a contract",
              id: "getting-started/working-with-smart-contracts/calling-contract-methods",
            },
          ],
        },
      ],
    },
    {
      type: "category",
      label: "Guides",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Debugging smart contracts",
          id: "guides/cometa-and-debugging",
        },
        {
          type: "doc",
          label: "Migrating existing dApps to =nil;",
          id: "guides/app-migration",
        },
        {
          type: "doc",
          label: "Adapting design patterns",
          id: "guides/design-patterns",
        },
        {
          type: "category",
          label: "Architecture",
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: "doc",
              label: "Opcode compatibility between Ethereum and =nil;",
              id: "guides/architecture/ethereum-nil-diffs",
            },
          ],
        },
      ],
    },
    {
      type: "category",
      label: "Tools",
      collapsed: false,
      collapsible: true,
      items: [
        {
          type: "category",
          label: "=nil; CLI",
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: "doc",
              label: "Usage",
              id: "tools/nil-cli/usage",
            },
            {
              type: "doc",
              label: "Error handling",
              id: "tools/nil-cli/error-handling",
            },
          ],
        },
        {
          type: "category",
          label: "Solidity library",
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: "doc",
              label: "Function modifiers",
              id: "tools/sol-library/func-modifiers",
            },
            {
              type: "doc",
              label: "Pre-compiles",
              id: "tools/sol-library/pre-compiles",
            },
          ],
        },
        {
          type: "category",
          label: "Block explorer",
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: "doc",
              label: "Usage",
              id: "tools/block-explorer/usage",
            },
          ],
        },
      ],
    },
    {
      type: "category",
      label: "Core concepts",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Accounts",
          id: "core-concepts/accounts",
        },
        {
          type: "doc",
          label: "Transaction lifecycle",
          id: "core-concepts/transaction-lifecycle",
        },
        {
          type: "doc",
          label: "Shards and parallelized execution",
          id: "core-concepts/shards-parallel-execution",
        },
      ],
    },
    {
      type: "category",
      label: "Reference",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "category",
          label: "=nil; CLI reference",
          items: [
            {
              type: "autogenerated",
              dirName: "reference/cli-reference",
            },
          ],
        },
        {
          type: "category",
          label: "Nil.js",
          link: {
            type: "doc",
            id: "reference/client/index",
          },
          items: [
            {
              type: "autogenerated",
              dirName: "reference/client",
            },
          ],
        },
      ],
    },
    {
      type: "category",
      label: "Specification",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Principles",
          id: "specification/principles",
        },
        {
          type: "doc",
          label: "Finality",
          id: "specification/finality",
        },
        {
          type: "doc",
          label: "Data availability",
          id: "specification/data-availability",
        },
        {
          type: "doc",
          label: "Sequencing",
          id: "specification/sequencing",
        },
        {
          type: "doc",
          label: "Contract placement",
          id: "specification/contract-co-location",
        },
      ],
    },
    {
      type: "doc",
      label: "Legal provisions",
      id: "legal",
    },
    {
      type: "link",
      href: "https://t.me/NilDevnetTokenBot",
      label: "Join Telegram",
    },
  ],
  vids: [
    {
      type: "doc",
      label: "Uniswap v2 =nil;",
      id: "vids/uniswap-v2",
    },
  ],
  migrationGuides: [
    {
      type: "category",
      label: "January 2025 releases",
      collapsed: false,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "29 January 2025 guide",
          id: "migration-guides/january-2901-2025-release",
        },
        {
          type: "doc",
          label: "14 January 2025 guide",
          id: "migration-guides/january-1401-2025-release",
        },
      ],
    },
    {
      type: "category",
      label: "December 2024 releases",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "17 December 2024 guide",
          id: "migration-guides/december-1712-2024-release",
        },
        {
          type: "doc",
          label: "04 December 2024 guide",
          id: "migration-guides/december-0412-2024-release",
        },
      ],
    },
    {
      type: "category",
      label: "November 2024 releases",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "20 November 2024 guide",
          id: "migration-guides/november-2011-2024-release",
        },
        {
          type: "doc",
          label: "06 November 2024 guide",
          id: "migration-guides/november-0611-2024-release",
        },
      ],
    },
    {
      type: "category",
      label: "October 2024 releases",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "15 October 2024 guide",
          id: "migration-guides/october-1510-2024-release",
        },
        {
          type: "doc",
          label: "02 October 2024 guide",
          id: "migration-guides/october-0110-2024-release",
        },
      ],
    },
    {
      type: "category",
      label: "September 2024 releases",
      collapsible: true,
      collapsed: true,
      items: [
        {
          type: "doc",
          label: "17 September 2024 guide",
          id: "migration-guides/september-1709-2024-release",
        },
        {
          type: "doc",
          label: "03 September 2024 guide",
          id: "migration-guides/september-2024-release",
        },
      ],
    },
  ],
};
