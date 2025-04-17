import {
  type Queries,
  type RenderOptions,
  render,
} from "@testing-library/react";
import { TestsLayout } from "./TestsLayout";
import type { ReactNode } from "react";

const renderWithLayout = <
  Q extends Queries,
  Container extends Element | DocumentFragment = HTMLElement,
  BaseElement extends Element | DocumentFragment = Container
>(
  ui: ReactNode,
  options?: RenderOptions<Q, Container, BaseElement>
) => render(ui, { wrapper: TestsLayout, ...options });

export { renderWithLayout };
