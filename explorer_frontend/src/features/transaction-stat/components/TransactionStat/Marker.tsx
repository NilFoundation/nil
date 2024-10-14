import { styled } from "styletron-react";

// biome-ignore lint/suspicious/noExplicitAny: <explanation>
const Marker = styled("div", (props: any) => ({
  width: "8px",
  height: "8px",
  borderRadius: "50%",
  backgroundColor: props.$color,
}));

export { Marker };
