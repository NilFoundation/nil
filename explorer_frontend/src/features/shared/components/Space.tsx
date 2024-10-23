import React from "react";
import { useStyletron } from "baseui";

export type SpaceSize = "small" | "medium" | "large";
export type Direction = "horizontal" | "vertical";

const sizeMap: Record<SpaceSize, string> = {
  small: "6px",
  medium: "8px",
  large: "12px",
};

type SpaceProps = {
  children: React.ReactNode;
  direction?: Direction;
  size?: SpaceSize;
  style?: React.CSSProperties;
  delimiter?: (key: string) => React.ReactNode;
};

export const Space = ({
  children,
  size = "medium",
  direction = "horizontal",
  style,
  delimiter,
}: SpaceProps) => {
  const [css] = useStyletron();
  return (
    <div
      className={css({
        display: "flex",
        flexDirection: direction === "horizontal" ? "row" : "column",
        gap: sizeMap[size],
      })}
      style={style}
    >
      {children &&
        React.Children.map(children, (child, index) => {
          return (
            <>
              {child}
              {index !== React.Children.count(children) - 1 &&
                child &&
                delimiter &&
                delimiter(`${index}`)}
            </>
          );
        })}
    </div>
  );
};
