import { EditorView } from "@codemirror/view";
import { CodeField, ParagraphSmall, TAG_KIND, TAG_SIZE, Tag } from "@nilfoundation/ui-kit";
import { Alert } from "baseui/icon";
import type { OnChangeHandler } from "baseui/tabs";
import { ethers } from "ethers";
import type { FC } from "react";
import { useEffect, useState } from "react";
import type { Key } from "react";
import { useStyletron } from "styletron-react";
import { addressRoute, blockRoute } from "../../routing";
import { Divider, Info, InfoBlock, Link, addHexPrefix, formatShard, measure } from "../../shared";
import { TokenDisplay } from "../../shared/components/Token";
import type { Transaction } from "../types/Transaction";
import { InlineCopyButton } from "./InlineCopyButton";

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
  const [activeKey, setActiveKey] = useState<Key>("0");
  const onChangeHandler: OnChangeHandler = (currentKey) => {
    setActiveKey(currentKey.activeKey);
  };

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
      <Info label="Tokens:" value={<TokenDisplay token={tx.token} />} />
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
      <Info label="Transaction payload (bytecode):" value={<Bytecode tx={tx} />} />
    </InfoBlock>
  );
};

const Default = ({ tx }: { tx: Transaction }) => {
  const [signature, setSignature] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  // Check if there is no input data
  if (!tx.method || tx.method.length === 0) {
    return <ParagraphSmall>No input data</ParagraphSmall>;
  }

  const inputData = addHexPrefix(tx.method);

  if (!ethers.isHexString(inputData)) {
    return <ParagraphSmall>Invalid hex string</ParagraphSmall>;
  }

  if (inputData.length < 10) {
    return <ParagraphSmall>Input data too short for function call</ParagraphSmall>;
  }

  const selector = inputData.slice(0, 10);

  // Define known function signatures (selector -> signature)
  const knownSignatures: { [key: string]: string } = {
    "0xa9059cbb": "transfer(address to, uint256 value)",
    "0x23b872dd": "transferFrom(address from, address to, uint256 value)",
    "0x095ea7b3": "approve(address spender, uint256 value)",
  };

  useEffect(() => {
    const fetchSignature = async () => {
      if (knownSignatures[selector]) {
        setSignature(knownSignatures[selector]);
      } else {
        setIsLoading(true);
        try {
          const response = await fetch(
            `https://www.4byte.directory/api/v1/signatures/?hex_signature=${selector}`,
          );
          const data = await response.json();
          if (data.results && data.results.length > 0) {
            setSignature(data.results[0].text_signature);
          } else {
            setSignature(null);
          }
        } catch (error) {
          console.error("Error fetching signature:", error);
          setSignature(null);
        } finally {
          setIsLoading(false);
        }
      }
    };

    fetchSignature();
  }, [selector]);

  if (isLoading) {
    return <ParagraphSmall>Fetching signature...</ParagraphSmall>;
  }

  if (!signature) {
    return <ParagraphSmall>Unknown function: {selector}</ParagraphSmall>;
  }

  try {
    const iface = new ethers.Interface([`function ${signature}`]);
    const functionName = signature.split("(")[0];
    const decoded = iface.decodeFunctionData(functionName, inputData);
    const func = iface.getFunction(functionName)!;

    const params = func.inputs.map((input: ethers.ParamType, index: number) => {
      const value = decoded[index];
      let formattedValue: string;
      if (input.type === "address") {
        formattedValue = addHexPrefix(value.toLowerCase());
      } else if (input.type.startsWith("uint") || input.type.startsWith("int")) {
        formattedValue = value.toString();
      } else {
        formattedValue = value.toString();
      }
      const paramName = input.name || `[${index}]`;
      return `${paramName}: ${formattedValue}`;
    });

    return (
      <div>
        <ParagraphSmall>
          <strong>Function:</strong> {signature}
        </ParagraphSmall>
        <ParagraphSmall>
          <strong>Method ID:</strong> {selector}
        </ParagraphSmall>
        <ParagraphSmall>
          <strong>Parameters:</strong>
        </ParagraphSmall>
        {params.map((param: string, index: number) => (
          <ParagraphSmall
            key={`param-${param}-${
              // biome-ignore lint/suspicious/noArrayIndexKey: <explanation>
              index
            }`}
          >
            {param}
          </ParagraphSmall>
        ))}
      </div>
    );
  } catch (error) {
    console.error("Error decoding input data:", error);
    return <ParagraphSmall>Error decoding input data</ParagraphSmall>;
  }
};

const Bytecode = ({ tx }: { tx: Transaction }) => {
  const [css] = useStyletron();
  return tx.method && tx.method.length > 0 ? (
    <CodeField
      extensions={[EditorView.lineWrapping]}
      code={tx.method}
      className={css({ marginTop: "0ch" })}
      codeMirrorClassName={css({ maxHeight: "300px", overflow: "scroll" })}
    />
  ) : (
    <ParagraphSmall>No bytecode</ParagraphSmall>
  );
};
