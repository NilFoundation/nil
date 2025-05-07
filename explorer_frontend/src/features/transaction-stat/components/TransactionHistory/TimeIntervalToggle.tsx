import { ToggleGroup } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { changeTimeInterval } from "../../models/model";
import { TimeInterval } from "../../types/TimeInterval";
import { styles as s } from "./styles";

type TimeIntervalToggleProps = {
  timeInterval: TimeInterval;
};

const timeIntervalOptions = [
  { key: TimeInterval.OneDay, label: "Today" },
  { key: TimeInterval.Days14, label: "14 days" },
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
