import { ParagraphLarge } from "baseui/typography";
import { ErrorBoundary } from "react-error-boundary";
import { useStyletron } from "styletron-react";
import { InfoContainer } from "../../../shared";
import { Chart } from "./Chart";
import { styles } from "./styles";

const ErrorView = () => {
  const [css] = useStyletron();

  return (
    <div className={css(styles.errorViewContainer)}>
      <ParagraphLarge>
        An error occurred while displaying the chart. Please try again later.
      </ParagraphLarge>
    </div>
  );
};

export const TransactionHistory = () => {
  return (
    <InfoContainer title="Transactions history">
      <ErrorBoundary fallback={<ErrorView />}>
        <Chart />
      </ErrorBoundary>
    </InfoContainer>
  );
};
