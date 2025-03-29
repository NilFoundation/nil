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
          label: "Fundamentals",
          id: "fundamentals",
        },
        {
          type: "doc",
          label: "Prerequisites",
          id: "prerequisites",
        },
        {
          type: "doc",
          label: "Quickstart",
          id: "nil-101",
        },
        {
          type: "doc",
          label: "Running the cluster locally",
          id: "nild",
        },
      ],
    },
    {
      type: "category",
      label: "Key principles",
      collapsed: false,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Account abstraction",
          id: "key-principles/account-abstraction",
        },
        {
          type: "doc",
          label: "Async execution",
          id: "key-principles/async-execution",
        },
        {
          type: "doc",
          label: "Gas and value",
          id: "key-principles/gas-and-value",
        },
        {
          type: "doc",
          label: "Tokens",
          id: "key-principles/tokens",
        },
      ],
    },
    {
      type: "category",
      label: "=nil; Solidity library",
      collapsed: false,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Getting started",
          id: "smart-contracts/getting-started",
        },
        {
          type: "doc",
          label: "Async execution",
          id: "smart-contracts/handling-async-execution",
        },
        {
          type: "doc",
          label: "Working with tokens",
          id: "smart-contracts/tokens",
        },
        {
          type: "doc",
          label: "Gas forwarding",
          id: "smart-contracts/gas-forwarding",
        },
        {
          type: "doc",
          label: "Receiving external transactions",
          id: "smart-contracts/receiving-ext-transactions",
        },
        {
          type: "doc",
          label: "Function modifiers",
          id: "smart-contracts/func-modifiers",
        },
        {
          type: "doc",
          label: "Pre-compiles",
          id: "smart-contracts/pre-compiles",
        },
      ],
    },
    {
      type: "category",
      label: "Nil.js",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Getting started",
          id: "niljs/getting-started",
        },
        {
          type: "doc",
          label: "Reading and writing info",
          id: "niljs/reading-writing-info",
        },
        {
          type: "doc",
          label: "Creating a smart account",
          id: "niljs/creating-smart-account",
        },
        {
          type: "doc",
          label: "Working with tokens",
          id: "niljs/tokens",
        },
        {
          type: "category",
          label: "Working with smart contracts",
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: "doc",
              label: "Deploying a smart contract",
              id: "niljs/deploying-smart-contract",
            },
            {
              type: "doc",
              label: "Calling methods inside a contract",
              id: "niljs/calling-smart-contract",
            },
          ],
        },
      ],
    },
    {
      type: "category",
      label: "=nil; CLI",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Getting started",
          id: "nilcli/getting-started",
        },
        {
          type: "doc",
          label: "Reading and writing info",
          id: "nilcli/reading-writing-info",
        },
        {
          type: "doc",
          label: "Working with tokens",
          id: "nilcli/tokens",
        },
        {
          type: "category",
          label: "Working with smart contracts",
          collapsed: true,
          collapsible: true,
          items: [
            {
              type: "doc",
              label: "Deploying a smart contract",
              id: "nilcli/deploying-smart-contract",
            },
            {
              type: "doc",
              label: "Calling methods inside a contract",
              id: "nilcli/calling-smart-contract",
            },
          ],
        },
        {
          type: "doc",
          label: "Error handling",
          id: "nilcli/error-handling",
        },
      ],
    },
    {
      type: "category",
      label: "Hardhat",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Getting started",
          id: "hardhat/getting-started",
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
      type: "doc",
      label: "Block explorer",
      id: "block-explorer/usage",
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
      label: "Cluster architecture",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "Sharding",
          id: "architecture/sharding",
        },
        {
          type: "doc",
          label: "Finality",
          id: "architecture/finality",
        },
        {
          type: "doc",
          label: "Data availability",
          id: "architecture/data-availability",
        },
        {
          type: "doc",
          label: "Sequencing",
          id: "architecture/sequencing",
        },
        {
          type: "doc",
          label: "Contract placement",
          id: "architecture/contract-co-location",
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
      label: "March 2025 releases",
      collapsed: false,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "03 March 2025 guide",
          id: "migration-guides/march-1803-2025-release",
        },
      ],
    },
    {
      type: "category",
      label: "February 2025 releases",
      collapsed: true,
      collapsible: true,
      items: [
        {
          type: "doc",
          label: "03 February 2025 guide",
          id: "migration-guides/february-0302-2025-release",
        },
      ],
    },
    {
      type: "category",
      label: "January 2025 releases",
      collapsed: true,
      collapsible: true,
      items: [
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
