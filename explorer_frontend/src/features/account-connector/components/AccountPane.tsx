import { useStyletron } from "styletron-react";
import { COLORS, LabelSmall } from "@nilfoundation/ui-kit";
import { ErrorBoundary } from "react-error-boundary";
import { styles } from "./styles";
import { AccountContent } from "./AccountContent";

const AccountPane = () => {
  const [css] = useStyletron();

  return (
    <div className={css(styles.container)}>
      <ErrorBoundary fallback={<LabelSmall color={COLORS.red200}>There is a problem with account</LabelSmall>}>
        <AccountContent />
      </ErrorBoundary>
    </div>
  );
};

export { AccountPane };
