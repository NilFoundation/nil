import { PRIMITIVE_COLORS, SPACE } from "@nilfoundation/ui-kit";
import { ParagraphSmall } from "baseui/typography";
import { useMobile } from "../hooks/useMobile";
import { useStyletron } from "baseui";

export type InfoProps = {
  label: string;
  value: React.ReactNode;
  mobileDouble?: boolean;
};

export const Info = ({ label, value, mobileDouble }: InfoProps) => {
  const [isMobile] = useMobile();
  const [css] = useStyletron();
  if (isMobile) {
    const content = (
      <>
        <ParagraphSmall
          color={PRIMITIVE_COLORS.gray400}
          className={css({
            display: "inline-block",
            marginBottom: mobileDouble ? SPACE[4] : SPACE[8],
          })}
        >
          {label}
        </ParagraphSmall>
        {typeof value === "string" ? (
          <ParagraphSmall
            color={PRIMITIVE_COLORS.gray100}
            className={css({
              display: "inline-block",
              wordBreak: "break-all",
            })}
          >
            {value}
          </ParagraphSmall>
        ) : (
          <div>{value}</div>
        )}
      </>
    );
    if (mobileDouble) {
      return (
        <div
          className={css({
            gridColumn: "1 / span 2",
          })}
        >
          {content}
        </div>
      );
    }
    return content;
  }
  return (
    <>
      <ParagraphSmall color={PRIMITIVE_COLORS.gray400}>{label}</ParagraphSmall>
      {typeof value === "string" ? (
        <ParagraphSmall color={PRIMITIVE_COLORS.gray100}>{value}</ParagraphSmall>
      ) : (
        <div>{value}</div>
      )}
    </>
  );
};
