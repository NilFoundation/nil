import { pino } from 'pino';

const logger = pino({
    level: 'silent',
    transport: {
        target: 'pino-pretty', // Pretty-prints logs for CLI
        options: {
            colorize: true,
            translateTime: true,
        },
    },
});

export default logger;