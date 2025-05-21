import { LabelLarge } from "baseui/typography";
import { useStyletron } from "styletron-react";
import { styles as s } from "./styles";

type LegendProps = {
  value: string;
};

export const Legend = ({ value }: LegendProps) => {
  const [css] = useStyletron();

  return (
    <div className={css(s.legend)}>
      <LabelLarge
        className={css({
          fontSize: "24px",
          lineHeight: "32px",
        })}
      >
        {value}
      </LabelLarge>
    </div>
  );
};
