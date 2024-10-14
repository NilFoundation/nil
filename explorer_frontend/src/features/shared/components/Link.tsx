import { COLORS } from "@nilfoundation/ui-kit";
import type { RouteParams } from "atomic-router";
import { Link as AtomicLink } from "atomic-router-react";
import { useStyletron } from "baseui";
import type { StylesObject } from "..";
import { expandProperty } from "inline-style-expand-shorthand";
import type { ExtendedRoute } from "../../routing";

export type LinkProps = {
  // biome-ignore lint/suspicious/noExplicitAny: <explanation>
  to: string | ExtendedRoute<any>;
  params: RouteParams;
  children: React.ReactNode;
  className?: string;
};

const styles: StylesObject = {
  link: {
    color: COLORS.blue200,
    textDecoration: "none",
    ...expandProperty("transition", "color 0.15s ease-in"),
    ":hover": {
      color: COLORS.blue400,
    },
  },
};

export const Link = ({ to, params, children, className }: LinkProps) => {
  const [css] = useStyletron();
  return (
    <AtomicLink to={to} params={params} className={`${css(styles.link)} ${className ? className : ""}`}>
      {children}
    </AtomicLink>
  );
};
