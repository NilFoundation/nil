import { COLORS } from "@nilfoundation/ui-kit";
import { expandProperty } from "inline-style-expand-shorthand";
import type { StyleObject } from "styletron-react";
import { getMobileStyles } from "../../../styleHelpers";

const buttonReset: StyleObject = {
	border: "none",
	padding: 0,
	margin: 0,
	background: "none",
	cursor: "pointer",
};

export const styles = {
	container: {
		display: "flex",
		justifyContent: "center",
		flexDirection: "column",
		alignItems: "center",
		width: "416px",
		height: "46px",
		backgroundColor: COLORS.gray900,
		...expandProperty("borderRadius", "8px"),
		position: "relative",
		...getMobileStyles({
			width: "auto",
			backgroundColor: "transparent",
		}),
	},
	indicator: {
		width: "16px",
		height: "16px",
		...expandProperty("borderRadius", "4px"),
		backgroundColor: COLORS.gray200,
		flexShrink: 0,
	},
	activeIndicator: {
		backgroundColor: COLORS.green200,
	},
	icon: {
		flexShrink: 0,
	},
	label: {
		width: "calc(100% - 16px - 24px - 8px - 16px)",
		color: COLORS.gray200,
	},
	account: {
		display: "flex",
		height: "100%",
		justifyContent: "center",
		gap: "8px",
		alignItems: "center",
		width: "100%",
		flexGrow: 0,
		...buttonReset,
		...expandProperty("padding", "0 16px"),
	},
	menu: {
		listStyle: "none",
		...expandProperty("borderRadius", "8px"),
		backgroundColor: COLORS.gray800,
		...getMobileStyles({
			maxWidth: "250px",
		}),
	},
	menuItem: {
		display: "flex",
		justifyContent: "space-between",
		alignItems: "center",
		gap: "8px",
		...expandProperty("padding", "8px 0"),
		...expandProperty("transition", "background-color 0.15s"),
		minHeight: "46px",
	},
	disabledMenuItem: {
		opacity: 0.5,
	},
	divider: {
		borderTop: `1px solid ${COLORS.gray600}`,
		width: "100%",
		...expandProperty("margin", "4px 0"),
	},
} as const;
