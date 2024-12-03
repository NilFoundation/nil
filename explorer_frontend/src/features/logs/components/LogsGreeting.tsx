import { COLORS, MonoParagraphMedium } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { getRuntimeConfigOrThrow } from "../../runtime-config";

type LogsGreetingProps = {
  className?: string;
};

const LogsGreeting: FC<LogsGreetingProps> = ({ className }) => {
  const [css] = useStyletron();
  const liCn = css({
    position: "relative",
    display: "flex",
    ":before": {
      content: "'•'",
      color: COLORS.gray400,
      width: "16px",
      display: "inline-block",
      position: "relative",
    },
  });

  return (
    <div className={className}>
      <MonoParagraphMedium color={COLORS.gray400}>Welcome to =nil; sandbox!</MonoParagraphMedium>
      <MonoParagraphMedium color={COLORS.gray400}>
        You can use this terminal to:
      </MonoParagraphMedium>
      <ul
        className={css({
          marginBottom: "32px",
          marginLeft: "16px",
        })}
      >
        <li className={liCn}>
          <MonoParagraphMedium color={COLORS.gray400}>
            Check transactions details and start debugging.
          </MonoParagraphMedium>
        </li>
        <li className={liCn}>
          <MonoParagraphMedium color={COLORS.gray400}>
            Compile and deploy smart contracts.
          </MonoParagraphMedium>
        </li>
      </ul>
      <MonoParagraphMedium color={COLORS.gray400}>Check essential:</MonoParagraphMedium>
      <ul
        className={css({
          marginLeft: "32px",
        })}
      >
        <li className={liCn}>
          <MonoParagraphMedium color={COLORS.gray400}>
            <a
              className={css({
                textDecoration: "underline",
              })}
              href={getRuntimeConfigOrThrow().DOCUMENTATION_URL}
              target="_blank"
              rel="noreferrer"
            >
              Documentation
            </a>
          </MonoParagraphMedium>
        </li>
        <li className={liCn}>
          <MonoParagraphMedium color={COLORS.gray400}>
            <a
              className={css({
                textDecoration: "underline",
              })}
              href={getRuntimeConfigOrThrow().SANDBOX_NILJS_URL}
              target="_blank"
              rel="noreferrer"
            >
              Nil.js
            </a>
          </MonoParagraphMedium>
        </li>
      </ul>
    </div>
  );
};

export { LogsGreeting };
