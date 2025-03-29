import { useStyletron } from "baseui";
import { expandProperty } from "inline-style-expand-shorthand";
import type { ElementType } from "react";
import { useMobile } from "../../hooks/useMobile";

type CardProps = {
  children: React.ReactNode;
  as?: ElementType;
  className?: string;
  transparent?: boolean;
};

export const Card = ({ children, as: Element = "div", className = "", transparent }: CardProps) => {
  const [css, theme] = useStyletron();
  const styles = {
    card: {
      ...expandProperty("borderRadius", "16px"),
      ...expandProperty("padding", "32px"),
      backgroundColor: theme.colors.backgroundPrimary,
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
