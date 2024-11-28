import { COLORS, encodeInlineSvg } from "@nilfoundation/ui-kit";
import type { FC, ReactNode } from "react";
import { useStyletron } from "styletron-react";

type LogTitleWithDetailsProps = {
  title: ReactNode;
  details: ReactNode;
};

const chevronSvg = `<svg viewBox="0 0 32 32" fill="${COLORS.gray200}" xmlns="http://www.w3.org/2000/svg">
    <path d="M16 22.0001L6 12.0001L7.4 10.6001L16 19.2001L24.6 10.6001L26 12.0001L16 22.0001Z"></path>
  </svg>`;

export const LogTitleWithDetails: FC<LogTitleWithDetailsProps> = ({ title, details }) => {
  const [css] = useStyletron();

  return (
    <details
      className={css({
        ":first-child[open] > summary::after": {
          transform: "rotate(180deg)",
        },
      })}
    >
      <summary
        className={css({
          cursor: "pointer",
          display: "flex",
          alignItems: "center",
          paddingBottom: "8px",
          listStyle: "none",
          "::-webkit-details-marker": {
            display: "none",
          },
          "::after": {
            content: '""',
            width: "16px",
            height: "16px",
            background: `url("${encodeInlineSvg(chevronSvg)}")`,
            backgroundSize: "cover",
            marginLeft: "1ch",
          },
        })}
      >
        {title}
      </summary>
      {details}
    </details>
  );
};
