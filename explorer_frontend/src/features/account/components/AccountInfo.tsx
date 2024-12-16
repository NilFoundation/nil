import { useUnit } from "effector-react";
import {
  $account,
  $accountCometaInfo,
  loadAccountCometaInfoFx,
  loadAccountStateFx,
} from "../model";
import {
  CodeField,
  HeadingXLarge,
  ParagraphSmall,
  SPACE,
  Skeleton,
  Spinner,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { addressRoute } from "../../routing";
import { InfoBlock } from "../../shared/components/InfoBlock";
import { Info } from "../../shared/components/Info";
import { measure } from "../../shared/utils/measure";
import { useEffect } from "react";
import { Divider } from "../../shared";
import { CurrencyDisplay } from "../../shared/components/Currency";
import { EditorView } from "@codemirror/view";
import { SolidityCodeField } from "../../shared/components/SolidityCodeField";
import { $cometaService } from "../../cometa/model";

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
  const [account, accountCometaInfo, isLoading, isLoadingCometaInfo, params, cometa] = useUnit([
    $account,
    $accountCometaInfo,
    loadAccountStateFx.pending,
    loadAccountCometaInfoFx.pending,
    addressRoute.$params,
    $cometaService,
  ]);
  const [css] = useStyletron();
  const sourceCode = accountCometaInfo?.sourceCode?.Compiled_Contracts;

  useEffect(() => {
    loadAccountStateFx(params.address);
    loadAccountCometaInfoFx({ address: params.address, cometaService: cometa });
  }, [params.address, cometa]);

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
            label="Source code"
            value={
              sourceCode?.length > 0 ? (
                <SolidityCodeField
                  code={sourceCode}
                  className={css({ marginTop: "1ch" })}
                  codeMirrorClassName={css({
                    maxHeight: "300px",
                    overflow: "scroll",
                    overscrollBehavior: "contain",
                  })}
                />
              ) : isLoadingCometaInfo ? (
                <div
                  className={css({
                    display: "flex",
                    justifyContent: "center",
                    alignItems: "center",
                    height: "300px",
                  })}
                >
                  <Spinner />
                </div>
              ) : (
                <ParagraphSmall>Not available</ParagraphSmall>
              )
            }
          />
          <Info
            label="Bytecode"
            value={
              account.code && account.code.length > 0 ? (
                <CodeField
                  extensions={[EditorView.lineWrapping]}
                  code={account.code}
                  className={css({ marginTop: "1ch" })}
                  codeMirrorClassName={css({
                    maxHeight: "300px",
                    overflow: "scroll",
                    overscrollBehavior: "contain",
                  })}
                />
              ) : (
                <ParagraphSmall>Not deployed</ParagraphSmall>
              )
            }
          />
        </InfoBlock>
      </div>
    );
  }

  // default
  return <AccountLoading />;
};
