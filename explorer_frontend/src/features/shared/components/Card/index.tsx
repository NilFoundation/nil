import { COLORS } from "@nilfoundation/ui-kit";
import { expandProperty } from "inline-style-expand-shorthand";
import type { ElementType } from "react";
import { useStyletron } from "styletron-react";
import { useMobile } from "../../hooks/useMobile";
import { useUnit } from "effector-react";
import { isTutorialPage } from "../../../code/model";

type CardProps = {
  children: React.ReactNode;
  as?: ElementType;
  className?: string;
  transparent?: boolean;
};



export const Card = ({ children, as: Element = "div", className = "", transparent }: CardProps) => {
  const isTutorial = useUnit(isTutorialPage);
  const styles = {
    card: {
      ...expandProperty("borderRadius", "16px"),
      ...expandProperty("padding", "32px"),
      backgroundColor: (isTutorial ? COLORS.blue800 : COLORS.gray900),
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
