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
import { useUnit } from "effector-react";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { $shardsAmount } from "../../../shards/models/model";
import { $shardIdIsValid, decrementShardId, incrementShardId } from "../../models/base";

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
  const [shardsAmount, shardIdIsValid] = useUnit([$shardsAmount, $shardIdIsValid]);
  const failedToGetShardsAmount = shardsAmount === -1;

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
          <FormControl label={`Shard ID (1 to ${shardsAmount})`} error={!shardIdIsValid}>
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
      {shardIdIsValid ? null : (
        <ParagraphXSmall color={COLORS.red400} marginTop="-8px">
          {`That Shard ID doesn't exist. Please select from 1 to ${shardsAmount}.`}
        </ParagraphXSmall>
      )}
      {failedToGetShardsAmount && (
        <ParagraphXSmall color={COLORS.red400}>
          Failed to get shards amount. No validation is applied.
        </ParagraphXSmall>
      )}
      <ParagraphXSmall color={COLORS.gray400}>
        <div>Choosing a shard can help reduce transaction gas fees.</div>
        <div>
          Learn how to select or check shards in the{" "}
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
