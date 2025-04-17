import { renderWithLayout } from "@test/unit/renderWithLayout";
import { screen } from "@testing-library/react";
import * as effectorReact from "effector-react";
import { describe, it, vi } from "vitest";
import { measure } from "../../shared/utils/measure";
import { AccountInfo } from "./AccountInfo";

// biome-ignore lint/suspicious/noExplicitAny: <explanation>
const getUseUnitReturnedValue = (args: any = {}): any => {
  return [
    args.account ?? { address: "0x123", balance: "1000" },
    args.accountCometaInfo ?? null,
    args.isLoading ?? false,
    args.isLoadingCometaInfo ?? false,
    args.params ?? { address: "0x123" },
    args.cometa ?? null,
  ];
};

vi.mock("../../routing", () => ({
  addressRoute: {
    $params: { getState: () => ({ address: "0x123" }) },
    $isOpened: { getState: () => true },
  },
}));

vi.mock("effector-react");
const mockUseUnit = vi.mocked(effectorReact.useUnit);

describe("AccountInfo", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders without crashing", () => {
    mockUseUnit.mockReturnValue(getUseUnitReturnedValue());

    renderWithLayout(<AccountInfo />);

    expect(screen.getByTestId("vitest-unit--account-container")).toBeInTheDocument();
  });

  it("renders skeleton when loading", () => {
    mockUseUnit.mockReturnValue(getUseUnitReturnedValue({ account: null, isLoading: true }));

    renderWithLayout(<AccountInfo />);

    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });

  it("renders account information when loaded", () => {
    mockUseUnit.mockReturnValue(
      getUseUnitReturnedValue({
        account: { address: "0x123", balance: "1000", tokens: [] },
      }),
    );

    renderWithLayout(<AccountInfo />);

    expect(screen.getByText("Address")).toBeInTheDocument();
    expect(screen.getByText("0x123")).toBeInTheDocument();
    expect(screen.getByText("Balance")).toBeInTheDocument();
    expect(screen.getByText(measure("1000"))).toBeInTheDocument();
    expect(screen.getByText("Tokens")).toBeInTheDocument();
    expect(screen.getByText("Bytecode")).toBeInTheDocument();
    expect(screen.getByText("Not available")).toBeInTheDocument();
  });

  it("renders fallback UI when source code is not available", () => {
    mockUseUnit.mockReturnValue(
      getUseUnitReturnedValue({
        accountCometaInfo: { sourceCode: { Compiled_Contracts: "" } },
      }),
    );

    renderWithLayout(<AccountInfo />);

    expect(screen.getByText("Source code")).toBeInTheDocument();
    expect(screen.getByText("Not available")).toBeInTheDocument();
  });

  it("shows spinner when cometa info is loading", () => {
    mockUseUnit.mockReturnValue(
      getUseUnitReturnedValue({
        isLoadingCometaInfo: true,
      }),
    );

    renderWithLayout(<AccountInfo />);

    expect(screen.getByTestId("vitest-unit--loading-cometa-info-spinner")).toBeInTheDocument();
  });
});
