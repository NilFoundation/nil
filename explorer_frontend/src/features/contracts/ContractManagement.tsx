import {
	BUTTON_KIND,
	BUTTON_SIZE,
	Button,
	CopyButton,
	ModalBody,
	ModalHeader,
	ParagraphMedium,
	SPACE,
	Tab,
	Tabs,
} from "@nilfoundation/ui-kit";
import { useUnit } from "effector-react";
import {
	$activeAppWithState,
	$activeKeys,
	$balance,
	$callParams,
	$callResult,
	$errors,
	$loading,
	$managementKey,
	$tokens,
	$txHashes,
	callMethod,
	sendMethod,
	setManagementPage,
	setParams,
	unlinkApp,
} from "./model";
import { useMemo } from "react";
import type { AbiFunction } from "abitype";
import { useStyletron } from "baseui";
import { Link } from "../shared";
import { addressRoute } from "../routing";
import { Method } from "./Method";

export const ContractManagement = () => {
	const [
		app,
		key,
		activeKeys,
		balance,
		tokens,
		callParams,
		callResult,
		loading,
		errors,
		txHashes,
	] = useUnit([
		$activeAppWithState,
		$managementKey,
		$activeKeys,
		$balance,
		$tokens,
		$callParams,
		$callResult,
		$loading,
		$errors,
		$txHashes,
	]);
	const [css] = useStyletron();
	const readFunctions: AbiFunction[] = useMemo(() => {
		if (!app) {
			return [];
		}
		return app.abi.filter((abiField) => {
			return (
				abiField.type === "function" && abiField.stateMutability === "view"
			);
		}) as AbiFunction[];
	}, [app]);
	const writeFunctions = useMemo(() => {
		if (!app) {
			return [];
		}
		return app.abi.filter((abiField) => {
			return (
				abiField.type === "function" && abiField.stateMutability !== "view"
			);
		}) as AbiFunction[];
	}, [app]);
	return (
		<>
			<ModalHeader>Contract {app?.name}</ModalHeader>
			<ModalBody>
				<div
					className={css({
						display: "flex",
						flexDirection: "column",
						gap: SPACE[4],
						flexGrow: 0,
						justifyContent: "flex-start",
						alignItems: "flex-start",
					})}
				>
					<ParagraphMedium
						className={css({
							display: "flex",
							gap: "8px",
							alignItems: "center",
						})}
					>
						Address:{" "}
						{app?.address && (
							<Link to={addressRoute} params={{ address: app.address }}>
								{app.address}
							</Link>
						)}
						{app?.address && <CopyButton textToCopy={app.address || ""} />}
						<Button
							size={BUTTON_SIZE.compact}
							kind={BUTTON_KIND.secondary}
							onClick={() => {
								if (app?.address)
									unlinkApp({ app: app.bytecode, address: app.address });
							}}
						>
							Remove app
						</Button>
					</ParagraphMedium>
					<ParagraphMedium>Balance: {balance.toString(10)}</ParagraphMedium>
					<ParagraphMedium>
						Tokens: {Object.keys(tokens).length === 0 && "No tokens"}
					</ParagraphMedium>
				</div>
				{Object.entries(tokens).map(([token, amount]) => {
					return (
						<ParagraphMedium key={token}>
							<Link to={addressRoute} params={{ address: token }}>
								{token}:
							</Link>{" "}
							{amount.toString(10)}
						</ParagraphMedium>
					);
				})}
				<Tabs
					activeKey={key}
					onChange={(page) => {
						setManagementPage(`${page.activeKey}`);
					}}
				>
					<Tab key="read" title="Read">
						<div
							className={css({
								display: "flex",
								flexDirection: "column",
								flexGrow: 0,
								flexShrink: 0,
								gap: SPACE[16],
							})}
						>
							{readFunctions.map((func, index) => {
								return (
									<Method
										key={func.name}
										func={func}
										index={index}
										handler={callMethod}
										isOpen={activeKeys[func.name]}
										error={errors[func.name] || undefined}
										result={callResult[func.name]}
										loading={loading[func.name] || false}
										params={callParams[func.name]}
										paramsHandler={(p) => {
											setParams(p);
										}}
									/>
								);
							})}
						</div>
					</Tab>
					<Tab key="write" title="Write">
						{writeFunctions.map((func, index) => {
							return (
								<Method
									key={func.name}
									func={func}
									index={index}
									handler={sendMethod}
									isOpen={activeKeys[func.name]}
									error={errors[func.name] || undefined}
									loading={loading[func.name] || false}
									txHash={txHashes[func.name] || undefined}
									params={callParams[func.name]}
									paramsHandler={(p) => {
										setParams(p);
									}}
								/>
							);
						})}
					</Tab>
				</Tabs>
			</ModalBody>
		</>
	);
};
