import { SPACE } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import type { ReactNode } from "react";
import { useMobile } from "..";
import type { StyleObject } from "styletron-react";

type InfoBlockProps = { children: ReactNode; className?: string };

const desktopStyles: StyleObject = {
  display: "grid",
  gridTemplateColumns: "171px 1fr",
  gap: SPACE[32],
  rowGap: SPACE[16],
};

export const InfoBlock = ({ children, className }: InfoBlockProps) => {
  const [css] = useStyletron();

  const [isMobile] = useMobile();

  const mobileStyles: StyleObject = {
    display: "grid",
    gridTemplateColumns: isMobile ? "auto 1 fr" : "max-content 1fr",
    gap: SPACE[12],
    rowGap: SPACE[isMobile ? 24 : 16],
    wordBreak: "keep-all",
  };

  const cn =
    css({
      ...(isMobile ? mobileStyles : desktopStyles),
    }) + (className ? ` ${className}` : "");

  return <div className={cn}>{children}</div>;
};
