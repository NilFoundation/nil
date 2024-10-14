import { COLORS, LabelMedium, LabelSmall } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { getCurrencyIcon, isBaseCurrency } from "../../currencies";

type TokenProps = {
  name: string;
  balance: string;
  isMain: boolean;
};

const Token: FC<TokenProps> = ({ name, balance, isMain = false }) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        paddinTop: "12px",
        paddingBottom: "12px",
        display: "grid",
        gridTemplateColumns: "48px auto auto",
        gridTemplateRows: "24px 24px",
        columnGap: "20px",
        rowGap: "4px",
        height: "48px",
        width: "100%",
      })}
    >
      <div
        className={css({
          width: "48px",
          height: "48px",
          backgroundColor: COLORS.gray50,
          ...expandProperty("borderRadius", "8px"),
        })}
      >
        {isBaseCurrency(name) && (
          <img
            className={css({
              width: "100%",
              height: "100%",
              ...expandProperty("borderRadius", "8px"),
            })}
            src={getCurrencyIcon(name)}
            alt={`${name} icon`}
          />
        )}
      </div>
      <LabelMedium
        className={css({
          color: COLORS.gray50,
          gridColumn: "2 / 3",
          gridRow: "1 / 2",
        })}
      >
        {name}
      </LabelMedium>
      <LabelSmall
        className={css({
          color: isMain ? COLORS.green200 : COLORS.gray200,
          gridColumn: "2 / 3",
          gridRow: "2 / 3",
        })}
      >
        {isMain ? "Main" : "Token"}
      </LabelSmall>
      <LabelMedium
        className={css({
          color: COLORS.gray50,
          gridColumn: "3 / 4",
          gridRow: "1 / 3",
          display: "flex",
          alignItems: "center",
          justifyContent: "flex-end",
        })}
      >
        {balance}
      </LabelMedium>
    </div>
  );
};

export { Token };
