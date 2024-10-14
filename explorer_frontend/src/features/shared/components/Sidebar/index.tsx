import { useStyletron } from "styletron-react";
import { styles } from "./styles";
import { Navigation } from "./Navigation";
import { useMobile } from "../../hooks/useMobile";

export const Sidebar = () => {
  const [css] = useStyletron();

  const [isMobile] = useMobile();

  if (isMobile) return null;

  return (
    <aside className={css(styles.aside)}>
      <Navigation />
    </aside>
  );
};
