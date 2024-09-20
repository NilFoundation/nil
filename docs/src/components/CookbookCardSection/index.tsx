import React, { type ReactNode, type PropsWithChildren } from "react";
import Link from "@docusaurus/Link";
import styles from "./styles.module.css";
import Timer from "@site/static/img/timerWhite.png";

type CardBadge = {
	label: string;
};

type CardBadgeProp = CardBadge | CardBadge[];

type BadgesProps = {
	badges: CardBadgeProp;
};

export const CookbookBadges = [
	{
		label: "Solidity tutorial",
	},
	{
		label: "Cross-shard comms",
	},
  {
    label: "Nil.js tutorial"
  },
	{
		label: "Hardhat plugin",
	},
];

export function CookbookCardSection({
	id,
	title,
	children,
	description,
	HeadingTag = "h3",
}: {
	id?: string;
	title: string;
	children: ReactNode;
	description?: ReactNode;
	HeadingTag?: keyof JSX.IntrinsicElements;
	className?: string;
}) {
	return (
		<div className={styles.cookbookSection}>
			{title && <HeadingTag id={id ?? title}>{title}</HeadingTag>}
			{description && (
				<p className="cookbook-section-description">{description}</p>
			)}
			<div className={styles.cookbookSectionContent}>{children}</div>
		</div>
	);
}

export function CookbookCard({
	id,
	title,
	description,
	to,
	tag,
	badges,
}: PropsWithChildren<{
	id?: string;
	title: string;
	description?: string;
	to: string;
	tag?: {
		label: string;
		description: string;
	};
	className?: string;
	badges: BadgesProps;
}>) {
	const badgesArray = Array.isArray(badges) ? badges : [badges];
	return (
		<Link to={to} className={styles.cookbookCard}>
			<div className={styles.cookbookCardContent}>
				{badges && (
					<div className={styles.cookbookCardBadges}>
						{badgesArray.map((badge) => (
							// biome-ignore lint/correctness/useJsxKeyInIterable: <explanation>
							<span
								className={styles.cookbookCardBadge}
								style={{ backgroundColor: "#d7d7d7" }}
							>
								{badge.label}
							</span>
						))}
					</div>
				)}
				<div className={styles.cookbookCardContentTitle} id={id && title}>
					{title}
				</div>
				{description && (
					<div className={styles.cookbookCardContentDescription}>
						{description}
					</div>
				)}
			</div>
			{tag && (
				<div className="tag absolute right-0 top-0 h-16 w-16">
					<span
						className={`${styles.cookbookCardTag} absolute right-[-28px] top-[-2px] w-[80px] rotate-45 transform py-1 text-center font-semibold`}
						title={tag.description}
					>
						<img src={Timer} alt="Timer" className={styles.timerImage} />
						{tag.label}
					</span>
				</div>
			)}
		</Link>
	);
}
