import { COLORS, ErrorPage } from "@nilfoundation/ui-kit";
import { RouterProvider } from "atomic-router-react";
import { ErrorBoundary } from "react-error-boundary";
import { useStyletron } from "styletron-react";
import { router } from "./features/routing";
import { RoutesView } from "./features/routing";
import type { StylesObject } from "./features/shared";

const styles: StylesObject = {
  main: {
    position: "relative",
    minHeight: "100vh",
    width: "100%",
    display: "flex",
    flexDirection: "column",
    background: COLORS.black,
    alignItems: "center",
    justifyContent: "flex-start",
  },
};

export const App = () => {
  const [css] = useStyletron();

  return (
    <>
      <style>
        {`
          * {
            scrollbar-width: none;
            scrollbar-color: #8A8A8A #f1f1f1;
            cursor: pointer !important;
            border-radius: 10px;
          }

          *:hover {
            scrollbar-width: thin;
          }
        `}
      </style>
      <main className={css(styles.main)}>
        <ErrorBoundary
          fallback={
            <ErrorPage
              errorDescription="Something went wrong... Please reload the page or try again later."
              errorCode={500}
              redirectPath="/"
              redirectTitle="Explorer page"
            />
          }
        >
          <RouterProvider router={router}>
            <RoutesView />
          </RouterProvider>
        </ErrorBoundary>
      </main>
    </>
  );
};
