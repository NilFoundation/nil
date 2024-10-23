import { useUnit } from "effector-react";
import {
	$activeAppWithState,
	$contractWithState,
	$contracts,
	closeApp,
} from "./model";
import "./init";
import {
	BUTTON_KIND,
	BUTTON_SIZE,
	SPACE,
	Button,
	Card,
	COLORS,
	LabelMedium,
	Spinner,
	ArrowUpIcon,
	Modal,
} from "@nilfoundation/ui-kit";
import { useStyletron } from "styletron-react";
import { compileCodeFx } from "../code/model";
import { useMobile } from "../shared";
import { getMobileStyles } from "../../styleHelpers";
import { LayoutComponent, setActiveComponent } from "../../pages/sandbox/model";
import { Contract } from "./Contract";
import { ContractManagement } from "./ContractManagement";
import { DeployForm } from "./DeployForm";

export const Contracts = () => {
	const [activeApp, deployedApps, contracts, compilingContracts] = useUnit([
		$activeAppWithState,
		$contractWithState,
		$contracts,
		compileCodeFx.pending,
	]);
	const [css] = useStyletron();
	const [isMobile] = useMobile();

	return (
		<div
			className={css({
				display: "flex",
				flexDirection: "column",
				height: "calc(100vh - 89px)",
				...getMobileStyles({
					height: "calc(100vh - 109px)",
				}),
			})}
		>
			{isMobile && (
				<div
					className={css({
						display: "flex",
						gap: "12px",
						marginBottom: SPACE[12],
						alignItems: "center",
					})}
				>
					<Button
						className={css({
							width: "32px",
							height: "32px",
						})}
						overrides={{
							Root: {
								style: {
									paddingLeft: 0,
									paddingRight: 0,
								},
							},
						}}
						kind={BUTTON_KIND.secondary}
						size={BUTTON_SIZE.compact}
						onClick={() => setActiveComponent(LayoutComponent.Code)}
					>
						<ArrowUpIcon
							size={12}
							className={css({
								transform: "rotate(-90deg)",
							})}
						/>
					</Button>
					<LabelMedium color={COLORS.gray50}>Contracts</LabelMedium>
				</div>
			)}
			<Card
				overrides={{
					Root: {
						style: {
							maxWidth: "none",
							height: "100%",
							backgroundColor: COLORS.gray900,
							overflow: "auto",
							overscrollBehavior: "contain",
						},
					},
					Contents: {
						style: {
							height: "100%",
							maxWidth: "none",
							width: "100%",
						},
					},
					Body: {
						style: {
							height: "100%",
							width: "100%",
							maxWidth: "none",
						},
					},
				}}
			>
				<Modal
					isOpen={!!activeApp}
					onClose={() => {
						closeApp();
					}}
					size={"80vw"}
				>
					{activeApp?.address ? <ContractManagement /> : <DeployForm />}
				</Modal>
				{contracts.length === 0 && (
					<div
						className={css({
							height: "100%",
							display: "flex",
							alignItems: "center",
							justifyContent: "center",
							paddingLeft: "25%",
							paddingRight: "25%",
							textAlign: "center",
						})}
					>
						{compilingContracts ? (
							<Spinner />
						) : (
							<LabelMedium color={COLORS.gray400}>
								Compile the code to handle smart contracts.
							</LabelMedium>
						)}
					</div>
				)}
				{contracts.map((contract, i) => {
					return (
						<Contract
							key={`${contract.bytecode}-${i}`}
							contract={contract}
							deployedApps={deployedApps.filter(
								(app) => app.bytecode === contract.bytecode,
							)}
						/>
					);
				})}
			</Card>
		</div>
	);
};
