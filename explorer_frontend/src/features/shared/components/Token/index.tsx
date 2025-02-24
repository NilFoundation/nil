import type { Token } from "@nilfoundation/explorer-backend/daos/transactions";
import { ParagraphSmall } from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { addressRoute } from "../../../routing";
import { getTokenSymbolByAddress } from "../../../tokens/utils";
import { Link } from "../Link";

export const TokenDisplay = ({ token }: { token: Token[] }) => {
  const [css] = useStyletron();
  if (!token || token.length === 0) {
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
      {token.map(({ token, balance }) => (
        <>
          <ParagraphSmall key={token}>
            <Link to={addressRoute} params={{ address: token }}>
              {getTokenSymbolByAddress(token)}
            </Link>
          </ParagraphSmall>
          <ParagraphSmall key={token + balance}>{balance}</ParagraphSmall>
        </>
      ))}
    </div>
  );
};
