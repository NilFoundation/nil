import { ToggleGroup } from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { styles as s } from "./styles";
import { changeTimeInterval } from "../../models/model";
import { TimeInterval } from "../../types/TimeInterval";
import type { FC } from "react";

type TimeIntervalToggleProps = {
  timeInterval: TimeInterval;
};

const timeIntervalOptions = [
  { key: TimeInterval.OneMinute, label: TimeInterval.OneMinute },
  { key: TimeInterval.FifteenMinutes, label: TimeInterval.FifteenMinutes },
  { key: TimeInterval.ThirtyMinutes, label: TimeInterval.ThirtyMinutes },
  { key: TimeInterval.OneDay, label: TimeInterval.OneDay },
] as const;

export const TimeIntervalToggle: FC<TimeIntervalToggleProps> = ({ timeInterval }) => {
  const [css] = useStyletron();

  return (
    <ToggleGroup
      options={timeIntervalOptions}
      value={[timeInterval]}
      onChange={([val]) => {
        val && changeTimeInterval(val);
      }}
      className={css(s.timeIntervalToggle)}
    />
  );
};
