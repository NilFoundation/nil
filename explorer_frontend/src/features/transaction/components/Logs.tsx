import { COLORS, CopyButton, Tag } from "@nilfoundation/ui-kit";
import { ParagraphLarge, ParagraphSmall } from "baseui/typography";
import { expandProperty } from "inline-style-expand-shorthand";
import type { FC } from "react";
import { type StyleObject, useStyletron } from "styletron-react";
import { addressRoute } from "../../routing";
import { Card, Link, addHexPrefix } from "../../shared";
import type { TransactionLog } from "../types/TransactionLog";
import { getTableContainerStyles } from "../../../styleHelpers";

type LogsProps = {
  logs: TransactionLog[];
};

const styles = {
  logsContainer: {
    display: "flex",
    flexDirection: "column",
    gap: "16px",
  },
  contaier: {
    display: "grid",
    gridTemplateColumns: "1fr 5fr",
    gap: "16px",
    height: "100%",
    width: "100%",
    flexGrow: 1,
    ...getTableContainerStyles(),
  },
  infoContainer: {
    display: "flex",
    flexDirection: "row",
    alignItems: "flex-start",
    gap: "1ch",
    height: "1lh",
  },
  data: {
    ...expandProperty("borderRadius", "16px"),
    ...expandProperty("padding", "16px"),
    display: "flex",
    whiteSpace: "pre-wrap",
    overflowWrap: "break-word",
    wordWrap: "break-word",
    wordBreak: "break-word",
    backgroundColor: COLORS.gray800,
  },
  topic: {
    display: "flex",
    flexDirection: "row",
    gap: "1ch",
  },
} as const;

export const Logs: FC<LogsProps> = ({ logs }) => {
  const [css] = useStyletron();

  return (
    <>
      {logs.length === 0 ? (
        <ParagraphLarge>No logs</ParagraphLarge>
      ) : (
        <div className={css(styles.logsContainer)}>
          {logs.map((log) => (
            <Card key={log.transaction_hash} scrollable>
              <div className={css(styles.contaier)}>
                <ParagraphSmall color={COLORS.gray400}>Address:</ParagraphSmall>
                <div className={css(styles.infoContainer)}>
                  <ParagraphSmall>
                    <Link
                      to={addressRoute}
                      params={{ address: addHexPrefix(log.address.toLowerCase()) }}
                    >
                      {addHexPrefix(log.address.toLowerCase())}
                    </Link>
                  </ParagraphSmall>
                  <CopyButton textToCopy={addHexPrefix(log.address.toLowerCase())} />
                </div>
                <ParagraphSmall color={COLORS.gray400}>Topics:</ParagraphSmall>
                <div>{getTopics(log, css)}</div>
                <ParagraphSmall color={COLORS.gray400}>Data:</ParagraphSmall>
                <div className={css(styles.data)}>{log.data}</div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </>
  );
};

const getTopics = (log: TransactionLog, css: (style: StyleObject) => string) => {
  const limit = log.topics_count;

  return Array.from({ length: limit }, (_, i) => {
    const topic = log[`topic${i + 1}` as keyof TransactionLog];
    return (
      // biome-ignore lint/suspicious/noArrayIndexKey: <explanation>
      <div className={css(styles.topic)} key={log.transaction_hash + i}>
        <Tag>{i}</Tag>
        <div>{topic}</div>
      </div>
    );
  });
};
