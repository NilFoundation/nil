import {
  BUTTON_KIND,
  ButtonIcon,
  COLORS,
  FormControl,
  Input,
  MinusIcon,
  ParagraphXSmall,
  PlusIcon,
} from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { decrementShardId, incrementShardId } from "../../model";

type ShardIdInputProps = {
  shardId: number | null;
  setShardId: (shardId: number | null) => void;
  disabled?: boolean;
};

const btnOverrides = {
  Root: {
    style: {
      width: "46px",
      height: "46px",
      marginBottom: "16px",
    },
  },
};

export const ShardIdInput: FC<ShardIdInputProps> = ({ shardId, setShardId, disabled }) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        marginBottom: "24px",
        gap: "4px",
      })}
    >
      <div
        className={css({
          display: "flex",
          gap: "8px",
          alignItems: "flex-end",
        })}
      >
        <div>
          <FormControl label="Shard ID">
            <Input
              value={shardId?.toString() ?? ""}
              onChange={(e) => {
                const { value } = e.target;
                if (value === "") {
                  setShardId(null);
                  return;
                }

                const sId = Number.parseInt(value, 10);
                if (Number.isNaN(sId)) {
                  setShardId(null);
                  return;
                }
                setShardId(sId);
              }}
              type="number"
              disabled={disabled}
              overrides={{
                Input: {
                  style: {
                    "::-webkit-outer-spin-button": {
                      WebkitAppearance: "none",
                      margin: 0,
                    },
                    "::-webkit-inner-spin-button": {
                      WebkitAppearance: "none",
                      margin: 0,
                    },
                    "-moz-appearance": "textfield",
                  },
                },
                Root: {
                  style: {
                    width: "145px",
                  },
                },
              }}
            />
          </FormControl>
        </div>
        <ButtonIcon
          kind={BUTTON_KIND.secondary}
          icon={<MinusIcon size={16} />}
          onClick={() => decrementShardId()}
          overrides={btnOverrides}
          disabled={shardId === null || shardId <= 1 || disabled}
        />
        <ButtonIcon
          kind={BUTTON_KIND.secondary}
          icon={<PlusIcon size={16} />}
          onClick={() => incrementShardId()}
          overrides={btnOverrides}
          disabled={disabled}
        />
      </div>
      <ParagraphXSmall color={COLORS.gray400} marginTop="-16px">
        <div>Choosing a shard can help reduce transaction gas fees.</div>
        <div>
          Learn{" "}
          <a
            className={css({
              textDecoration: "underline",
            })}
            href={import.meta.env.VITE_EXPLORER_USAGE_DOCS_URL}
            target="_blank"
            rel="noreferrer"
          >
            how to select
          </a>{" "}
          or check shards in the{" "}
          <a
            href="https://explore.nil.foundation"
            className={css({
              textDecoration: "underline",
            })}
            target="_blank"
            rel="noreferrer"
          >
            Explorer
          </a>
          .
        </div>
      </ParagraphXSmall>
    </div>
  );
};
