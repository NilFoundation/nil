import { COLORS } from "@nilfoundation/ui-kit";
import { HeadingLarge, ParagraphSmall } from "baseui/typography";
import { useStyletron } from "styletron-react";
import { EmptyList } from "../../shared";

export const EmptyTransaction = () => {
  const [css] = useStyletron();
  return (
    <div
      className={css({
        display: "grid",
        placeItems: "center",
        width: "100%",
        marginBlockStart: "1.5rem",
        marginBlockEnd: "2rem",
      })}
    >
      <EmptyList />
      <HeadingLarge className={css({ textAlign: "center" })}>
        No transactions have been made yet
      </HeadingLarge>
      <ParagraphSmall color={COLORS.gray300} className={css({ marginBlockEnd: "1.5rem" })}>
        This block is currently empty
      </ParagraphSmall>
    </div>
  );
};
