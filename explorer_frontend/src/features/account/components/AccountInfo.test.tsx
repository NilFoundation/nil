import { renderWithLayout } from "@test/unit/renderWithLayout";
import { screen } from "@testing-library/react";
import * as effectorReact from "effector-react";
import { describe, it } from "vitest";
import { measure } from "../../shared/utils/measure";
import { AccountInfo } from "./AccountInfo";
import { fork, type Scope, type TypeOfSource } from "effector";
import {
  $account,
  $accountCometaInfo,
  loadAccountCometaInfoFx,
  loadAccountStateFx,
} from "../model";
import { $cometaClient } from "../../cometa/model";
import "../init.ts";
import { router, addressRoute } from "../../routing";
import { RouterProvider } from "atomic-router-react";

const initScope = (args?: {
  account?: Partial<TypeOfSource<typeof $account>>;
  accountCometaInfo?: Partial<TypeOfSource<typeof $accountCometaInfo>>;
  isLoading?: boolean;
  isLoadingCometaInfo?: boolean;
  params?: Partial<TypeOfSource<typeof addressRoute.$params>>;
  cometa?: Partial<TypeOfSource<typeof $cometaClient>>;
}): Scope => {
  return fork({
    values: [
      [$account, args?.account ?? { address: "0x123", balance: "1000" }],
      [$accountCometaInfo, args?.accountCometaInfo ?? null],
      [loadAccountStateFx.pending, args?.isLoading ?? false],
      [loadAccountCometaInfoFx.pending, args?.isLoadingCometaInfo ?? false],
      [addressRoute.$params, args?.params ?? { address: "0x123" }],
      [addressRoute.$isOpened, true],
      [$cometaClient, args?.cometa ?? null],
    ],
  });
};

describe("AccountInfo", () => {
  it("renders without crashing", () => {
    const scope = initScope();
    renderWithLayout(
      <effectorReact.Provider value={scope}>
        <RouterProvider router={router}>
          <AccountInfo />
        </RouterProvider>
      </effectorReact.Provider>,
    );

    expect(screen.getByTestId("vitest-unit--account-container")).toBeInTheDocument();
  });

  it("renders skeleton when loading", () => {
    const scope = initScope({ isLoading: true });
    renderWithLayout(
      <effectorReact.Provider value={scope}>
        <RouterProvider router={router}>
          <AccountInfo />
        </RouterProvider>
      </effectorReact.Provider>,
    );

    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });

  it("renders account information when loaded", () => {
    const scope = initScope({
      account: { balance: "1000", tokens: [] },
    });

    renderWithLayout(
      <effectorReact.Provider value={scope}>
        <RouterProvider router={router}>
          <AccountInfo />
        </RouterProvider>
      </effectorReact.Provider>,
    );

    expect(screen.getByText("Address")).toBeInTheDocument();
    expect(screen.getByText("0x123")).toBeInTheDocument();
    expect(screen.getByText("Balance")).toBeInTheDocument();
    expect(screen.getByText(measure("1000"))).toBeInTheDocument();
    expect(screen.getByText("Tokens")).toBeInTheDocument();
    expect(screen.getByText("Bytecode")).toBeInTheDocument();
    expect(screen.getByText("Not available")).toBeInTheDocument();
  });

  it("renders fallback UI when source code is not available", () => {
    const scope = initScope({
      accountCometaInfo: {
        sourceCode: {
          Compiled_Contracts: "",
        },
      },
    });

    renderWithLayout(
      <effectorReact.Provider value={scope}>
        <RouterProvider router={router}>
          <AccountInfo />
        </RouterProvider>
      </effectorReact.Provider>,
    );

    expect(screen.getByText("Source code")).toBeInTheDocument();
    expect(screen.getByText("Not available")).toBeInTheDocument();
  });

  it("shows spinner when cometa info is loading", () => {
    const scope = initScope({
      isLoadingCometaInfo: true,
    });

    renderWithLayout(
      <effectorReact.Provider value={scope}>
        <RouterProvider router={router}>
          <AccountInfo />
        </RouterProvider>
      </effectorReact.Provider>,
    );

    expect(screen.getByTestId("vitest-unit--loading-cometa-info-spinner")).toBeInTheDocument();
  });
});
