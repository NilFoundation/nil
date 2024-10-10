import { useStyletron } from "styletron-react";
import { backLinkStyle, button, leftArrow } from "./styles";
import { Link, useRouter } from "atomic-router-react";
import {
  ArrowUpIcon,
  BUTTON_KIND,
  BUTTON_SIZE,
  Button,
  ChartIcon,
  CodeIcon,
  type LinkComponentRenderFunction,
  MENU_SIZE,
  Menu,
  COLORS,
} from "@nilfoundation/ui-kit";
import { explorerRoute } from "../../../routing/routes/explorerRoute";
import { useUnit } from "effector-react";
import type { Items, MenuOverrides } from "baseui/menu";
import { sandboxRoute } from "../../../routing";

const menuOverrides: MenuOverrides = {
  List: {
    style: {
      boxShadow: "none",
      maxWidth: "171px",
    },
  },
};

export const Navigation = () => {
  const [css] = useStyletron();
  const router = useRouter();

  const [activeRoute] = useUnit(router.$activeRoutes);
  const isMainPage = activeRoute === explorerRoute;
  const isSandbox = activeRoute === sandboxRoute;

  const items: Items = [
    {
      label: "Explorer",
      startEnhancer: <ChartIcon />,
      isHighlighted: isMainPage,
      linkComponent: (({ children, className }) => (
        <Link to={explorerRoute} className={className}>
          {children}
        </Link>
      )) as LinkComponentRenderFunction,
    },
    {
      label: "Sandbox",
      startEnhancer: <CodeIcon $color={COLORS.gray100} />,
      isHighlighted: isSandbox,
      linkComponent: (({ children, className }) => (
        <Link to={sandboxRoute} className={className}>
          {children}
        </Link>
      )) as LinkComponentRenderFunction,
    },
    {
      label: "Diagnostic",
      startEnhancer: <ChartIcon />,
      disabled: true,
    },
  ];

  if (!isMainPage) {
    return (
      <Link to={explorerRoute} className={css(backLinkStyle)}>
        <Button className={css(button)} kind={BUTTON_KIND.secondary} size={BUTTON_SIZE.compact}>
          <ArrowUpIcon size={13} className={css(leftArrow)} />
        </Button>
      </Link>
    );
  }

  return <Menu items={items} size={MENU_SIZE.small} overrides={menuOverrides} />;
};
