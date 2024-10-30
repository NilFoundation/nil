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

type ShardIdInputProps = {
  shardId: number;
  setShardId: (shardId: number) => void;
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

export const ShardIdInput: FC<ShardIdInputProps> = ({ shardId, setShardId }) => {
  const [css] = useStyletron();
  const increment = () => setShardId(shardId + 1);
  const decrement = () => setShardId(shardId - 1);

  return (
    <div
      className={css({
        display: "flex",
        flexDirection: "column",
        marginBottom: "16px",
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
              value={shardId}
              onChange={(e) => setShardId(Number.parseInt(e.target.value))}
              type="number"
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
          icon={<PlusIcon size={16} />}
          onClick={increment}
          overrides={btnOverrides}
        />
        <ButtonIcon
          kind={BUTTON_KIND.secondary}
          icon={<MinusIcon size={16} />}
          onClick={decrement}
          overrides={btnOverrides}
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
          >
            how to select
          </a>{" "}
          or check shards in the{" "}
          <a
            href="https://explore.nil.foundation"
            className={css({
              textDecoration: "underline",
            })}
          >
            Explorer
          </a>
          .
        </div>
      </ParagraphXSmall>
    </div>
  );
};
