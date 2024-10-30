import { COLORS, Card } from "@nilfoundation/ui-kit";
import type { FC, ReactNode } from "react";
import { expandProperty } from "inline-style-expand-shorthand";

type ResultProps = {
  children: ReactNode;
};

export const Result: FC<ResultProps> = ({ children }) => {
  return (
    <Card
      overrides={{
        Root: {
          style: {
            maxWidth: "none",
            backgroundColor: "transparent",
            ...expandProperty("border", `1px solid ${COLORS.gray800}`),
            ...expandProperty("borderRadius", "16px"),
          },
        },
      }}
    >
      {children}
    </Card>
  );
};
