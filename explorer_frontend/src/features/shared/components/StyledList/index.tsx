import { styled, useStyletron } from "styletron-react";
import { styles as s } from "./styles";
import { useRef } from "react";
import { useOnScreen } from "@ukorvl/react-on-screen";

type StyledListProps = {
  children: React.ReactNode;
  scrollable?: boolean;
};

export const StyledList = ({ children, scrollable = false }: StyledListProps) => {
  const [css] = useStyletron();
  const ref = useRef<HTMLDivElement>(null);
  const { isOnScreen } = useOnScreen({ ref, threshold: 0.1 });

  return (
    <ul className={css(s.getListStyles(!isOnScreen, scrollable))}>
      {children}
      <div ref={ref} className={css(s.dummy)} aria-hidden />
    </ul>
  );
};

const Item = styled("li", s.item);

StyledList.Item = Item;
