import { COLORS } from "@nilfoundation/ui-kit";

export const QuestionIcon = () => {
  return (
    <svg width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
      <title>Question icon</title>
      <path
        d="M8.99999 12V9.9375C10.05 9.9375 12.375 9 12.3 6.84375C12.2586 5.65457 11.7 3.75 9.29999 3.75C6.89999 3.75 6.29999 5.76974 6.29999 7.47837"
        stroke={COLORS.gray200}
        strokeWidth="1.8"
        strokeLinecap="square"
        strokeLinejoin="round"
      />
      <rect x="8.25" y="14.25" width="1.5" height="1.5" fill={COLORS.gray200} />
    </svg>
  );
};
