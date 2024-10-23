import {
	BUTTON_KIND,
	BUTTON_SIZE,
	Button,
	COLORS,
	CopyButton,
	LabelMedium,
} from "@nilfoundation/ui-kit";
import type { FC } from "react";
import {
	$codeSnippetHash,
	$shareCodeSnippetError,
	setCodeSnippetEvent,
	setCodeSnippetFx,
} from "./model";
import { useUnit } from "effector-react";
import { useStyletron } from "styletron-react";
import { Link } from "../shared/components/Link";
import { OverflowEllipsis } from "../shared";
import { sandboxWithHashRoute } from "../routing";

type ShareCodePanelProps = {
	disabled: boolean;
};

export const ShareCodePanel: FC<ShareCodePanelProps> = ({ disabled }) => {
	const [shareCodeSnippetPending, codeHash, shareCodeError] = useUnit([
		setCodeSnippetFx.pending,
		$codeSnippetHash,
		$shareCodeSnippetError,
	]);
	const [css] = useStyletron();
	const link = !codeHash
		? null
		: `${window.location.origin}/sandbox/${codeHash}`;
	const noWrapCn = css({
		whiteSpace: "nowrap",
	});

	return (
		<div
			className={css({
				display: "flex",
				alignItems: "center",
				justifyContent: "flex-start",
				gap: "16px",
			})}
		>
			<Button
				kind={BUTTON_KIND.secondary}
				size={BUTTON_SIZE.default}
				onClick={() => setCodeSnippetEvent()}
				disabled={disabled}
				isLoading={shareCodeSnippetPending}
				className={noWrapCn}
			>
				Share code
			</Button>
			{link && !shareCodeError && (
				<div
					className={css({
						display: "flex",
						alignItems: "center",
						justifyContent: "flex-start",
						gap: "1ch",
						width: "100%",
					})}
				>
					<LabelMedium className={noWrapCn} color={COLORS.gray200}>
						Link to share the code:
					</LabelMedium>
					<div
						className={css({
							maxWidth: "calc(50% - 200px)",
						})}
					>
						<Link to={sandboxWithHashRoute} params={{ snippetHash: codeHash }}>
							<OverflowEllipsis>{link}</OverflowEllipsis>
						</Link>
					</div>
					<CopyButton textToCopy={link} />
				</div>
			)}
			{shareCodeError && (
				<LabelMedium color={COLORS.red200}>
					An error occured while generating the link
				</LabelMedium>
			)}
		</div>
	);
};
