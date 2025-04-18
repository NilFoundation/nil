import type { FC, ReactNode } from "react";
import { useStyletron } from "styletron-react";

type InternalPageContainerProps = {
  children: ReactNode;
};

export const InternalPageContainer: FC<InternalPageContainerProps> = ({ children }) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        gridColumn: "1 / 4",
        // paddingLeft: isMobile ? "0" : "32px",
        // paddingRight: isMobile ? "0" : "32px",
      })}
    >
      {children}
    </div>
  );
};
