import { CopyButton, type CopyButtonProps } from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";

type InlineCopyButtonProps = CopyButtonProps;

export const InlineCopyButton = ({ className, ...rest }: InlineCopyButtonProps) => {
  const [css] = useStyletron();
  const cn = css({
    display: "inline-block",
    paddingRight: 0,
    paddingLeft: 0,
    paddingTop: 0,
    paddingBottom: 0,
    marginLeft: "0.5ch",
  });

  return <CopyButton className={cn} {...rest} />;
};
