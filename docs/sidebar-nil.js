export default {
    nil: [
        {
            type: 'category',
            label: 'Introduction',
            collapsed: false,
            collapsible: true,
            items: [
                {
                    type: 'doc',
                    label: 'Overview',
                    id: 'intro',
                },
                {
                    type: 'doc',
                    label: 'Thesis',
                    id: 'thesis'
                },
                {
                    type: 'doc',
                    label: 'Principles',
                    id: 'principles'
                }
            ]
        },
        {
            type: 'category',
            label: 'Getting started',
            collapsed: false,
            collapsible: true,
            items: [
                {
                    type: 'doc',
                    label: 'Prerequisites',
                    id: 'getting-started/prerequisites'
                },
                {
                    type: 'doc',
                    label: '=nil; 101',
                    id: 'getting-started/nil-101'
                },
                {
                    type: 'category',
                    label: 'Essentials',
                    collapsed: true,
                    collapsible: true,
                    items: [
                        {
                            type: 'doc',
                            label: 'Creating a wallet',
                            id: 'getting-started/essentials/creating-a-wallet'
                        },
                        {
                            type: 'doc',
                            label: 'Tokens and multi-currency support',
                            id: 'getting-started/essentials/tokens-multi-currency'
                        },
                        {
                            type: 'doc',
                            label: 'Reading and writing to =nil;',
                            id: 'getting-started/essentials/reading-writing-info'
                        },
                        {
                            type: 'doc',
                            label: 'Receiving external messages',
                            id: 'getting-started/essentials/receiving-ext-messages'
                        },
                        {
                            type: 'doc',
                            label: 'Handling async execution',
                            id: 'getting-started/essentials/handling-async-execution'
                        },
                        {
                            type: 'doc',
                            label: 'Gas and value',
                            id: 'getting-started/essentials/gas-and-value'
                        },
                    ]
                },
                {
                    type: 'category',
                    label: 'Working with smart contracts',
                    collapsed: true,
                    collapsible: true,
                    items: [
                        {
                            type: 'doc',
                            label: 'Writing a smart contract',
                            id: 'getting-started/working-with-smart-contracts/writing-a-contract'
                        },
                        {
                            type: 'doc',
                            label: 'Deploying a smart contract',
                            id: 'getting-started/working-with-smart-contracts/deploying-a-contract'
                        },
                        {
                            type: 'doc',
                            label: 'Calling methods inside a contract',
                            id: 'getting-started/working-with-smart-contracts/calling-contract-methods'
                        },
                    ]
                }
            ]
        },
        {
            type: 'category',
            label: 'Tools',
            collapsed: false,
            collapsible: true,
            items: [
                {
                    type: 'category',
                    label: '=nil; CLI',
                    collapsed: true,
                    collapsible: true,
                    items: [
                        {
                            type: 'doc',
                            label: 'Usage',
                            id: 'tools/nil-cli/usage'
                        },
                        {
                            type: 'doc',
                            label: 'Error handling',
                            id: 'tools/nil-cli/error-handling'
                        }
                    ]
                },
                {
                    type: 'category',
                    label: 'Hardhat plugin',
                    collapsed: true,
                    collapsible: true,
                    items: [
                        {
                            type: 'doc',
                            label: 'Usage',
                            id: 'tools/hardhat-plugin/usage'
                        }
                    ]
                },
                {
                    type: 'category',
                    label: 'Solidity library',
                    collapsed: true,
                    collapsible: true,
                    items: [
                        {
                            type: 'doc',
                            label: 'Function modifiers',
                            id: 'tools/sol-library/func-modifiers'
                        },
                        {
                            type: 'doc',
                            label: 'Pre-compiles',
                            id: 'tools/sol-library/pre-compiles'
                        }]
                },
                {
                    type: 'category',
                    label: 'Block explorer',
                    collapsed: true,
                    collapsible: true,
                    items: [
                        {
                            type: 'doc',
                            label: 'Usage',
                            id: 'tools/block-explorer/usage'
                        },
                    ]
                }
            ]
        },
        {
            type: 'category',
            label: 'Core concepts',
            collapsed: true,
            collapsible: true,
            items: [
                {
                    type: 'doc',
                    label: 'Accounts',
                    id: 'core-concepts/accounts',
                },
                {
                    type: 'doc',
                    label: 'Transaction lifecycle',
                    id: 'core-concepts/transaction-lifecycle',
                },
                {
                    type: 'doc',
                    label: 'Shards and parallelized execution',
                    id: 'core-concepts/shards-parallel-execution',
                },
            ]
        },
        {
            type: 'category',
            label: 'Guides',
            collapsed: true,
            collapsible: true,
            items: [
                {
                    type: 'category',
                    label: 'Architecture',
                    collapsed: true,
                    collapsible: true,
                    items: [
                        {
                            type: 'doc',
                            label: 'Opcode compatibility between Ethereum and =nil;',
                            id: 'guides/architecture/ethereum-nil-diffs'
                        },
                    ]
                }
            ]
        },
        {
            type: 'category',
            label: 'Reference',
            collapsed: true,
            collapsible: true,
            items: [
                {
                    type: 'doc',
                    label: '=nil; CLI reference',
                    id: 'reference/cli-reference'
                },
                {
                    type: 'category',
                    label: 'Nil.js',
                    link: {
                        type: 'doc',
                        id: 'reference/client/index',
                    },
                    items: [
                        {
                            type: 'autogenerated',
                            dirName: 'reference/client'
                        }
                    ]
                },
            ]
        },
        {
            type: 'category',
            label: 'Specification',
            collapsed: true,
            collapsible: true,
            items: [
                {
                    type: 'doc',
                    label: 'Finality',
                    id: 'specification/finality',
                },
                {
                    type: 'doc',
                    label: 'Data availability',
                    id: 'specification/data-availability',
                },
                {
                    type: 'doc',
                    label: 'Sequencing',
                    id: 'specification/sequencing',
                },
                {
                    type: 'doc',
                    label: 'Contract placement',
                    id: 'specification/contract-co-location',
                },
            ]
        },



    ]
}
