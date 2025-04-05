import {
  COLORS,
  CopyButton,
  HeadingXLarge,
  ParagraphMedium,
  ParagraphSmall,
  SPACE,
  Skeleton,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "baseui";
import { useUnit } from "effector-react";
import { Info } from "../../shared/components/Info";
import { InfoBlock } from "../../shared/components/InfoBlock";
import { $block, loadBlockFx } from "../models/model";
import { BlockNavigation } from "./BlockNavigation";

export const BlockInfo = ({ className }: { className?: string }) => {
  const [blockInfo, isPending] = useUnit([$block, loadBlockFx.pending]);
  const [css] = useStyletron();

  if (isPending) {
    return (
      <div className={className}>
        <HeadingXLarge className={css({ marginBottom: SPACE[32] })}>Block</HeadingXLarge>
        <Skeleton animation rows={6} width={"300px"} height={"400px"} />
      </div>
    );
  }

  if (blockInfo) {
    return (
      <div className={className}>
        <InfoBlock>
          <Info label="Shard ID" value={blockInfo.shard_id.toString()} />
          <Info label="Height" value={<BlockNavigation blockInfo={blockInfo} />} />
          <Info label="Hash:" value={<HashCopy hash={`0x${blockInfo.hash.toLowerCase()}`} />} />
        </InfoBlock>
      </div>
    );
  }

  return (
    <div className={className}>
      <HeadingXLarge>Block</HeadingXLarge>
      <InfoBlock>
        <ParagraphMedium>Block not found</ParagraphMedium>
      </InfoBlock>
    </div>
  );
};

const HashCopy = ({ hash }: { hash: string }) => {
  const [css] = useStyletron();

  return (
    <div
      className={css({
        display: "flex",
        alignItems: "start",
        gap: SPACE[8],
      })}
    >
      <ParagraphSmall
        color={COLORS.gray100}
        className={css({
          display: "inline-block",
          wordBreak: "break-all",
        })}
      >
        {hash}
      </ParagraphSmall>
      <CopyButton textToCopy={hash} disabled={hash === ""} color={COLORS.gray100} />
    </div>
  );
};
