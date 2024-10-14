import dayjs from "dayjs";

export const formatUTCTimestamp = (dateTimestamp: number, format: string): string =>
  dayjs(dateTimestamp).format(format);
