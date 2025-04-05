import type { FC, ReactNode } from "react";
import { useStyletron } from "styletron-react";
import { useMobile } from "../hooks/useMobile";

type InternalPageContainerProps = {
  children: ReactNode;
};

export const InternalPageContainer: FC<InternalPageContainerProps> = ({ children }) => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  return (
    <div
      className={css({
        gridColumn: "1 / 4",
        paddingInlineStart: isMobile ? "0" : "2rem",
        paddingInlineEnd: isMobile ? "0" : "2rem",
      })}
    >
      {children}
    </div>
  );
};
