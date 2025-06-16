import { HeadingXLarge, HeadingXXLarge } from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import type { StylesObject } from "../..";
import Search from "../../../search/components/Search";
import { useMobile } from "../../hooks/useMobile";

type HeadingProps = {
  className?: string;
};

const s: StylesObject = {
  heading: {
    marginBottom: "4px",
  },
};

export const Heading = ({ className = "" }: HeadingProps) => {
  const [css] = useStyletron();

  const [isMobile] = useMobile();

  const TitleComponent = isMobile ? HeadingXLarge : HeadingXXLarge;

  return (
    <div
      className={`${css({
        display: "flex",
        alignItems: "flex-start",
        justifyContent: "center",
        flexDirection: "column",
        flex: 0,
        gap: "24px",
      })} ${className}`.trim()}
    >
      <TitleComponent className={css(s.heading)}>Explore =nil; Network</TitleComponent>
      <Search />
    </div>
  );
};
