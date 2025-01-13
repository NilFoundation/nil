import { pino } from "pino";
import pinoPretty from "pino-pretty";

const prettyTransport = pinoPretty({
  colorize: true,
  translateTime: true,
});

const logger = pino(prettyTransport);
logger.level = "silent";

export { logger };
