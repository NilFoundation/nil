import { HeadingXXLarge } from "baseui/typography";
import { styles as s } from "./styles";
import { useStyletron } from "styletron-react";

type LegendProps = {
  value: string;
};

export const Legend = ({ value }: LegendProps) => {
  const [css] = useStyletron();

  return (
    <div className={css(s.legend)}>
      <HeadingXXLarge>{value}</HeadingXXLarge>
    </div>
  );
};
