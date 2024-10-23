import { COLORS, LabelMedium, LabelSmall, LabelXSmall } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { expandProperty } from "inline-style-expand-shorthand";
import { getCurrencyIcon, getCurrencySymbolByAddress } from "../../currencies";
import { OverflowEllipsis } from "../../shared";

type TokenProps = {
  name: string;
  balance: string;
  isMain: boolean;
};

const Token: FC<TokenProps> = ({ name, balance, isMain = false }) => {
  const [css] = useStyletron();
  const src = getCurrencyIcon(getCurrencySymbolByAddress(name));

  return (
    <div
      className={css({
        paddinTop: "12px",
        paddingBottom: "12px",
        display: "grid",
        gridTemplateColumns: "48px 84px 200px",
        gridTemplateRows: "24px 24px",
        columnGap: "20px",
        rowGap: "4px",
        height: "48px",
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
        {src ? (
          <img
            className={css({
              width: "100%",
              height: "100%",
              ...expandProperty("borderRadius", "8px"),
            })}
            src={src}
            alt={`${name} icon`}
          />
        ) : (
          <LabelXSmall
            className={css({
              width: "48px",
              height: "48px",
              ...expandProperty("borderRadius", "8px"),
              ...expandProperty("padding", "16px 4px 16px 4px"),
              backgroundColor: COLORS.gray600,
              color: COLORS.gray300,
              display: "flex",
              justifyContent: "center",
              alignItems: "center",
            })}
          >
            {getCurrencySymbolByAddress(name)}
          </LabelXSmall>
        )}
      </div>
      <LabelMedium
        className={css({
          color: COLORS.gray50,
          gridColumn: "2 / 3",
          gridRow: "1 / 2",
          maxWidth: "84px",
          display: "block",
        })}
      >
        <OverflowEllipsis charsFromTheEnd={4}>{getCurrencySymbolByAddress(name)}</OverflowEllipsis>
      </LabelMedium>
      <LabelSmall
        className={css({
          gridColumn: "2 / 3",
          gridRow: "2 / 3",
        })}
        color={isMain ? COLORS.green200 : COLORS.gray400}
      >
        {isMain ? "Main" : "Token"}
      </LabelSmall>
      <LabelSmall
        className={css({
          color: COLORS.gray50,
          gridColumn: "3 / 4",
          gridRow: "1 / 3",
          alignItems: "center",
          justifyContent: "flex-end",
          width: "200px",
          display: "flex",
          overflow: "hidden",
        })}
      >
        {`${balance} ${getCurrencySymbolByAddress(name)}`}
      </LabelSmall>
    </div>
  );
};

export { Token };
