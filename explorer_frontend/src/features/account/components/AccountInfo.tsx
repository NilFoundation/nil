import { EditorView } from "@codemirror/view";
import {
  CodeField,
  HeadingXLarge,
  ParagraphSmall,
  SPACE,
  Skeleton,
  Spinner,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { useEffect } from "react";
import { $cometaClient } from "../../cometa/model";
import { addressRoute } from "../../routing";
import { Divider } from "../../shared";
import { Info } from "../../shared/components/Info";
import { InfoBlock } from "../../shared/components/InfoBlock";
import { SolidityCodeField } from "../../shared/components/SolidityCodeField";
import { TokenDisplay } from "../../shared/components/Token";
import { measure } from "../../shared/utils/measure";
import {
  $account,
  $accountCometaInfo,
  loadAccountCometaInfoFx,
  loadAccountStateFx,
} from "../model";

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
    $cometaClient,
  ]);
  const [css] = useStyletron();
  const sourceCode = accountCometaInfo?.sourceCode?.Compiled_Contracts;

  useEffect(() => {
    loadAccountStateFx(params.address);
    loadAccountCometaInfoFx({ address: params.address, cometaClient: cometa });
  }, [params.address, cometa]);

  if (isLoading) return <AccountLoading />;

  if (account) {
    return (
      <div data-testid="vitest-unit--account-container">
        <InfoBlock>
          <Info label="Address" value={params.address} />
          <Info label="Balance" value={measure(account.balance)} />
          <Divider />
          <Info label="Tokens" value={<TokenDisplay token={account.tokens} />} />
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
                  data-testid="vitest-unit--loading-cometa-info-spinner"
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

  return <AccountLoading />;
};
