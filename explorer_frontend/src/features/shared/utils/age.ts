import dayjs from "dayjs";
import RelativeTime from "dayjs/plugin/relativeTime";

dayjs.extend(RelativeTime); // type is inferred as dayjs.DayJS & AdvancedFormat & RelativeTime;

export const age = (timestamp: number, showDate = true) => {
  const dayObject = dayjs.unix(timestamp);

  if (showDate) {
    return dayObject.format("YYYY-MM-DD HH:mm:ss");
  }

  return dayObject.fromNow();
};
