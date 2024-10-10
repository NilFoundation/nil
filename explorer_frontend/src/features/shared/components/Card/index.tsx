import { useStyletron } from "styletron-react";
import type { ElementType } from "react";
import { useMobile } from "../../hooks/useMobile";
import { expandProperty } from "inline-style-expand-shorthand";
import { COLORS } from "@nilfoundation/ui-kit";

type CardProps = {
  children: React.ReactNode;
  as?: ElementType;
  className?: string;
  transparent?: boolean;
};

const styles = {
  card: {
    ...expandProperty("borderRadius", "16px"),
    ...expandProperty("padding", "32px"),
    backgroundColor: COLORS.gray900,
    display: "flex",
    flexDirection: "column",
    justifyContent: "center",
    alignItems: "flex-start",
    minWidth: "0",
  },
  mobileCard: {
    ...expandProperty("padding", "24px"),
  },
  transparentCard: {
    backgroundColor: "transparent",
    ...expandProperty("padding", "0"),
  },
} as const;

export const Card = ({ children, as: Element = "div", className = "", transparent }: CardProps) => {
  const [css] = useStyletron();

  const [isMobile] = useMobile();

  return (
    <Element
      className={`${css({
        ...styles.card,
        ...(isMobile ? styles.mobileCard : {}),
        ...(transparent ? styles.transparentCard : {}),
      })} ${className}`}
    >
      {children}
    </Element>
  );
};
