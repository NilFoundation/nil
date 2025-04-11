import { useStyletron } from "styletron-react";

export const EmptyList = () => {
  const [css] = useStyletron();
  return (
    <div className={css({ marginBlockStart: "1.5rem", marginBlockEnd: "2rem" })}>
      {/* biome-ignore lint/a11y/noSvgWithoutTitle: <explanation> */}
      <svg
        width="62"
        height="51"
        viewBox="0 0 62 51"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="M10.1 1.58066L1.19354 10.4871L10.1 19.3936"
          stroke="#F1F1F1"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M51.6634 31.2687L60.5698 40.1751L51.6634 49.0816"
          stroke="#F1F1F1"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M1.19348 10.4872H60.5698"
          stroke="#F1F1F1"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path
          d="M1.19348 40.1752H60.5698"
          stroke="#F1F1F1"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </div>
  );
};
