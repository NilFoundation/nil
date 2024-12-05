import type { Currency } from "@nilfoundation/explorer-backend/daos/transactions";
import { ParagraphSmall } from "@nilfoundation/ui-kit";
import { addressRoute } from "../../../routing";
import { Link } from "../Link";
import { useStyletron } from "styletron-react";

export const CurrencyDisplay = ({ currency }: { currency: Currency[] }) => {
  const [css] = useStyletron();
  if (!currency || currency.length === 0) {
    return <ParagraphSmall>No tokens</ParagraphSmall>;
  }

  return (
    <div
      className={css({
        display: "grid",
        gridTemplateColumns: "1fr 1fr",
        gridTemplateRows: "auto",
        gridGap: "12px",
      })}
    >
      {currency.map(({ currency, balance }) => (
        <>
          <ParagraphSmall key={currency}>
            <Link to={addressRoute} params={{ address: currency }}>
              {currency}
            </Link>
          </ParagraphSmall>
          <ParagraphSmall key={currency + balance}>{balance}</ParagraphSmall>
        </>
      ))}
    </div>
  );
};
