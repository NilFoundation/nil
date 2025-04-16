import { COLORS, LabelSmall } from "@nilfoundation/ui-kit";
import { ErrorBoundary } from "react-error-boundary";
import { useStyletron } from "styletron-react";
import { useMobile } from "../../shared/hooks/useMobile";
import { AccountContent } from "./AccountContent";
import { styles } from "./styles";

const AccountPane = () => {
  const [css] = useStyletron();
  const [isMobile] = useMobile();
  return (
    <div
      className={css({
        ...styles.container,
        width: isMobile ? "32px" : "100%",
        height: isMobile ? "32px" : "46px",
        marginTop: "",
      })}
    >
      <ErrorBoundary
        fallback={
          <LabelSmall color={COLORS.red200}>There is a problem with the smart account</LabelSmall>
        }
      >
        <AccountContent />
      </ErrorBoundary>
    </div>
  );
};

export { AccountPane };
