import { ArrowUpIcon, BUTTON_KIND, BUTTON_SIZE, ButtonIcon } from "@nilfoundation/ui-kit";
import type { FC } from "react";
import { useStyletron } from "styletron-react";
import { Link } from "../../shared/components/Link";
import { explorerRoute } from "../../routing";
import { QuestionButton } from "./QuestionButton";
import { HyperlinkButton } from "./HyperlinkButton";
import { ExamplesButton } from "./ExamplesButton";
import { useMobile } from "../../shared";

type CodeToolbarProps = {
  disabled: boolean;
};

export const CodeToolbar: FC<CodeToolbarProps> = ({ disabled }) => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();

  return (
    <div
      className={css({
        display: "flex",
        alignItems: "center",
        justifyContent: isMobile ? "flex-end" : "flex-start",
        gap: "8px",
        flexGrow: 1,
      })}
    >
      {!isMobile && (
        <Link
          to={explorerRoute}
          params={{}}
          className={css({
            marginRight: "auto",
          })}
        >
          <ButtonIcon
            icon={<ArrowUpIcon size={18} />}
            kind={BUTTON_KIND.secondary}
            size={BUTTON_SIZE.large}
            className={css({
              transform: "rotate(-90deg)",
            })}
          />
        </Link>
      )}
      <QuestionButton />
      <HyperlinkButton disabled={disabled} />
      <ExamplesButton disabled={disabled} />
    </div>
  );
};
