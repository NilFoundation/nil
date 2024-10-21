import {
	BUTTON_KIND,
	Button,
	COLORS,
	Card,
	ChevronDownIcon,
	ChevronUpIcon,
	NOTIFICATION_KIND,
	ParagraphMedium,
	SPACE,
	Notification,
} from "@nilfoundation/ui-kit";
import type { AbiFunction } from "abitype";
import { useStyletron } from "baseui";
import { toggleActiveKey, $valueInput, setValueInput } from "./model";
import { Link } from "../shared";
import { transactionRoute } from "../routing";
import { MethodInput } from "./MethodInput";
import { CurrencyInput, getCurrencySymbolByAddress } from "../currencies";
import { $balanceCurrency } from "../account-connector/models/model";
import { useUnit } from "effector-react";

export type MethodProps = {
	func: AbiFunction;
	isOpen: boolean;
	index: number;
	handler: (funcName: string) => void;
	error?: string;
	result?: unknown;
	loading?: boolean;
	txHash?: string;
	params?: Record<string, unknown>;
	paramsHandler: (params: {
		functionName: string;
		paramName: string;
		value: unknown;
	}) => void;
};

export const Method = ({
	func,
	isOpen,
	index,
	handler,
	error,
	result,
	loading,
	txHash,
	paramsHandler,
	params,
}: MethodProps) => {
	const [css] = useStyletron();
	const isPayable = func.stateMutability === "payable";
	const [currecyBalance, valueInput] = useUnit([$balanceCurrency, $valueInput]);
	const availiableCurencies = [
		{ currency: "MZK" },
		...Object.keys(currecyBalance ?? {}).map((currency) => ({
			currency: getCurrencySymbolByAddress(currency),
		})),
	];

	return (
		<div
			key={func.name}
			className={css({
				paddingBottom: SPACE[16],
				borderBottom: `1px solid ${COLORS.gray800}`,
				display: "flex",
				flexDirection: "column",
				flex: "0 0",
			})}
		>
			<Button
				kind={BUTTON_KIND.secondary}
				className={css({
					marginBottom: isOpen ? SPACE[16] : 0,
					flexGrow: 0,
				})}
				endEnhancer={isOpen ? <ChevronUpIcon /> : <ChevronDownIcon />}
				onClick={() => {
					toggleActiveKey(func.name);
				}}
				isLoading={loading}
			>
				{index}. {func.name}
			</Button>
			{isOpen && (
				<>
					{isPayable && (
						<div
							className={css({
								display: "flex",
								flexDirection: "column",
								gap: SPACE[8],
								alignItems: "center",
								justifyContent: "center",
								marginBottom: "16px",
								width: "50%",
							})}
						>
							<CurrencyInput
								disabled={loading}
								label="Attach value"
								className={css({
									paddingTop: "16px",
									width: "100%",
								})}
								currencies={availiableCurencies}
								onChange={({ amount, currency }) => {
									setValueInput({ amount, currency });
								}}
								value={valueInput}
							/>
							<Button disabled kind={BUTTON_KIND.tertiary}>
								+ Add value
							</Button>
						</div>
					)}
					{func.inputs.length > 0 && (
						<Card
							overrides={{
								Root: {
									style: {
										marginBottom: SPACE[16],
										maxWidth: "100%",
									},
								},
							}}
						>
							{func.inputs.map((input, index) => {
								const key = input.name || `${index}`;
								return (
									<MethodInput
										key={key}
										methodName={func.name}
										paramsHandler={paramsHandler}
										params={params}
										paramName={key}
										input={input}
									/>
								);
							})}
						</Card>
					)}
					<Button
						onClick={() => {
							handler(func.name);
						}}
						disabled={loading}
					>
						Call
					</Button>
					{result && (
						<Notification kind={NOTIFICATION_KIND.info}>
							<ParagraphMedium
								className={css({
									wordBreak: "break-all",
								})}
							>
								Result: {`${result}`}
							</ParagraphMedium>
						</Notification>
					)}
					{txHash && (
						<Notification kind={NOTIFICATION_KIND.positive}>
							Transaction sent with hash{" "}
							<Link
								to={transactionRoute}
								params={{ hash: txHash }}
								className={css({
									wordBreak: "break-all",
								})}
							>
								{txHash}
							</Link>
						</Notification>
					)}
					{error && (
						<Notification kind={NOTIFICATION_KIND.negative}>
							{error}
						</Notification>
					)}
				</>
			)}
		</div>
	);
};
