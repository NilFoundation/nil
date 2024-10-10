import { useStyletron } from "styletron-react";
import { leftArrow, styles } from "./styles";
import { ArrowUpIcon, BUTTON_KIND, BUTTON_SIZE, ButtonIcon } from "@nilfoundation/ui-kit";
import { Link } from "atomic-router-react";
import { useMobile } from "../../hooks/useMobile";
import type { ExtendedRoute } from "../../../routing";

type SidebarWithBackLinkProps = {
  to: ExtendedRoute;
};

export const SidebarWithBackLink = ({ to }: SidebarWithBackLinkProps) => {
  const [css] = useStyletron();

  const [isMobile] = useMobile();

  return (
    <aside className={css(isMobile ? styles.mobileAside : styles.aside)}>
      <Link to={to} className={css(styles.link)}>
        <ButtonIcon
          icon={<ArrowUpIcon />}
          kind={BUTTON_KIND.secondary}
          size={BUTTON_SIZE.large}
          className={css(leftArrow)}
        />
      </Link>
    </aside>
  );
};
