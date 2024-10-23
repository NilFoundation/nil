import { CodeField, ParagraphSmall, TAG_KIND, TAG_SIZE, Tag } from "@nilfoundation/ui-kit";
import { Divider, Info, InfoBlock, Link, addHexPrefix, formatShard, measure } from "../../shared";
import type { Transaction } from "../types/Transaction";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { addressRoute, blockRoute } from "../../routing";
import { CurrencyDisplay } from "../../shared/components/Currency";
import { InlineCopyButton } from "./InlineCopyButton";
import { Alert } from "baseui/icon";
import { EditorView } from "@codemirror/view";

type OverviewProps = {
  transaction: Transaction;
};

const styles = {
  infoContainer: {
    display: "flex",
    flexDirection: "row",
    alignItems: "center",
    gap: "1ch",
    height: "1lh",
  },
} as const;

export const Overview: FC<OverviewProps> = ({ transaction: tx }) => {
  const [css] = useStyletron();

  return (
    <InfoBlock>
      <Info
        label="Shard + Block height:"
        value={
          <div className={css(styles.infoContainer)}>
            <ParagraphSmall>
              <Link to={blockRoute} params={{ shard: tx.shard_id, id: tx.block_id }}>
                {formatShard(tx.shard_id.toString(), tx.block_id.toString())}
              </Link>
            </ParagraphSmall>
          </div>
        }
      />
      <Info
        label="Hash:"
        value={
          <div className={css(styles.infoContainer)}>
            <ParagraphSmall
              className={css({
                wordBreak: "break-all",
                overflowWrap: "anywhere",
              })}
            >
              {addHexPrefix(tx.hash)}
              <InlineCopyButton textToCopy={addHexPrefix(tx.hash)} />
            </ParagraphSmall>
          </div>
        }
      />
      <Info
        label="Nonce:"
        value={
          <div className={css(styles.infoContainer)}>
            <ParagraphSmall>{tx.seqno}</ParagraphSmall>
          </div>
        }
      />
      <Info
        label="Status:"
        value={
          <div className={css(styles.infoContainer)}>
            <ParagraphSmall>
              {tx.success ? (
                "Success"
              ) : (
                <div
                  className={css({
                    display: "flex",
                    flexDirection: "row",
                    alignItems: "center",
                    gap: "0.5ch",
                  })}
                >
                  Failed
                  <Tag kind={TAG_KIND.red} size={TAG_SIZE.s}>
                    <Alert />
                  </Tag>
                </div>
              )}
            </ParagraphSmall>
          </div>
        }
      />
      <Divider />
      <Info
        label="From:"
        value={
          <div className={css(styles.infoContainer)}>
            <ParagraphSmall
              className={css({
                wordBreak: "break-all",
                overflowWrap: "anywhere",
              })}
            >
              <Link to={addressRoute} params={{ address: addHexPrefix(tx.from.toLowerCase()) }}>
                {addHexPrefix(tx.from.toLowerCase())}
              </Link>
              <InlineCopyButton textToCopy={addHexPrefix(tx.from.toLowerCase())} />
            </ParagraphSmall>
          </div>
        }
      />
      <Info
        label="To:"
        value={
          <div className={css(styles.infoContainer)}>
            <ParagraphSmall
              className={css({
                wordBreak: "break-all",
                overflowWrap: "anywhere",
              })}
            >
              <Link to={addressRoute} params={{ address: addHexPrefix(tx.to).toLowerCase() }}>
                {addHexPrefix(tx.to).toLowerCase()}
              </Link>
              <InlineCopyButton textToCopy={addHexPrefix(tx.to).toLowerCase()} />
            </ParagraphSmall>
          </div>
        }
      />
      <Divider />
      <Info label="Tokens:" value={<CurrencyDisplay currency={tx.currency} />} />
      <Divider />
      <Info
        label="Value:"
        value={
          <div className={css(styles.infoContainer)}>
            <ParagraphSmall>{measure(tx.value)}</ParagraphSmall>
          </div>
        }
      />
      <Info
        label="Fee credit:"
        value={
          <div className={css(styles.infoContainer)}>
            <ParagraphSmall>{measure(tx.fee_credit)}</ParagraphSmall>
          </div>
        }
      />
      <Info
        label="Gas used:"
        value={
          <div className={css(styles.infoContainer)}>
            <ParagraphSmall>{`${tx.gas_used ?? 0}`}</ParagraphSmall>
          </div>
        }
      />
      <Divider />
      <Info
        label="Message payload (bytecode):"
        value={
          tx.method && tx.method.length > 0 ? (
            <CodeField
              extensions={[EditorView.lineWrapping]}
              code={tx.method}
              className={css({ marginTop: "1ch" })}
              codeMirrorClassName={css({ maxHeight: "300px", overflow: "scroll" })}
            />
          ) : (
            <ParagraphSmall>No bytecode</ParagraphSmall>
          )
        }
      />
    </InfoBlock>
  );
};
