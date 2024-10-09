import { HeadingMedium, InfoIcon } from "@nilfoundation/ui-kit";
import { styles as s } from "./styles";
import { useStyletron } from "styletron-react";

type InfoContainerProps = {
  title?: string;
  description?: string;
  children?: React.ReactNode;
  showInfoIcon?: boolean;
};

export const InfoContainer = ({ title, children, showInfoIcon = false }: InfoContainerProps) => {
  const [css] = useStyletron();

  return (
    <div className={css(s.container)}>
      <div className={css(s.heading)}>
        {title && <HeadingMedium>{title}</HeadingMedium>}
        {showInfoIcon && <InfoIcon />}
      </div>
      {children}
    </div>
  );
};
