/// <reference types="vite/client" />
/// <reference types="cypress" />
/// <reference types="@testing-library/jest-dom" />

declare module "*.sol" {
  const content: string;
  export default content;
}

declare module '*.md' {
  const content: string;
  export default content;
}


declare namespace Cypress {
  interface Chainable {
    compileCounterContract(): Chainable<Element>;
  }
}
