import { vi } from "vitest";

vi.mock("@nilfoundation/ui-kit", () => {
  return {
    createTheme: vi.fn(() => ({
      theme: {
        breakpoints: {},
        colors: {},
        typography: {},
      },
    })),
    SPACE: {},
    COLORS: {},
    PRIMITIVE_COLORS: {},
    Spinner: vi.fn(() => <div>Spinner</div>),
    ParagraphSmall: vi.fn(({ children }) => <p>{children}</p>),
    HeadingXLarge: vi.fn(({ children }) => <h1>{children}</h1>),
    Skeleton: vi.fn(() => (
      <div
        role="progressbar"
        aria-valuemax={100}
        aria-valuemin={0}
        aria-valuenow={5}
        tabIndex={0}
      />
    )),
  };
});
