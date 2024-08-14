import { defineConfig } from 'vitest/config'

export default defineConfig({
    test: {
        globals: true,
        sequence: {
            shuffle: false,
            concurrent: false,
        },
        poolOptions: {
            forks: {
                singleFork: true,
            },
        },
        server: {
            deps: {
                inline: [
                    "@nilfoundation/niljs",
                ]
            }
        }
    },
})