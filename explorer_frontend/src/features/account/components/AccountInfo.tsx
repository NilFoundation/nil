import { useUnit } from "effector-react";
import { $account, loadAccountStateFx } from "../models/model";
import { CodeField, HeadingXLarge, ParagraphSmall, SPACE, Skeleton } from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { addressRoute } from "../../routing";
import { InfoBlock } from "../../shared/components/InfoBlock";
import { Info } from "../../shared/components/Info";
import { measure } from "../../shared/utils/measure";
import { useEffect } from "react";
import { Divider } from "../../shared";
import { CurrencyDisplay } from "../../shared/components/Currency";
import { EditorView } from "@codemirror/view";

const AccountLoading = () => {
  const [css] = useStyletron();

  return (
    <div>
      <HeadingXLarge className={css({ marginBottom: SPACE[32] })}>Account</HeadingXLarge>
      <Skeleton animation rows={4} width="300px" height="400px" />
    </div>
  );
};

export const AccountInfo = () => {
  const [account, isLoading, params] = useUnit([$account, loadAccountStateFx.pending, addressRoute.$params]);
  const [css] = useStyletron();

  useEffect(() => {
    loadAccountStateFx(params.address);
  }, [params.address]);

  if (isLoading) return <AccountLoading />;

  if (account) {
    return (
      <div>
        <InfoBlock>
          <Info label="Address" value={params.address} />
          <Info label="Balance" value={measure(account.balance)} />
          <Divider />
          <Info label="Tokens" value={<CurrencyDisplay currency={account.currencies} />} />
          <Divider />
          <Info
            label="Code"
            value={account.code && account.code.length > 0 ? (
                  <CodeField
                    extensions={[EditorView.lineWrapping]}
                    code={account.code}
                    className={css({ marginTop: "1ch" })}
                    codeMirrorClassName={css({ maxHeight: "300px", overflow: "scroll" })}
                  />
                ) : (
                  <ParagraphSmall>Not deployed</ParagraphSmall>
                )}
          />
        </InfoBlock>
      </div>
    );
  }

  // default
  return <AccountLoading />;
};
