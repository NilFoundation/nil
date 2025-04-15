beforeEach(() => {
  cy.visit("/playground");
});

const counter = `contract Counter {
  int256 private count;
  function getCount() public view returns (int256) {
      return count;
  }
  function increment() public {
      count += 1;
  }
  function decrement() public {
      count -= 1;
  }
}`;

Cypress.Commands.add("compileCounterContract", () => {
  cy.get("[data-testid=code-field]").within(() => {
    cy.get('[role="textbox"]').invoke("text", counter);
  });

  cy.get("[data-testid='compile-run-button']").click();
});

describe("playground page", () => {
  it("the h3 contains the correct text", () => {
    cy.get("[data-testid=code-field]").should("exist");
  });
});

describe("code field", () => {
  it("a user can change the code", () => {
    cy.get("[data-testid=code-field]").within(() => {
      cy.get('[role="textbox"]').invoke("text", "nil is good");
    });
  });

  it("invalid code compilation error", () => {
    cy.get("[data-testid=code-field]").within(() => {
      cy.get('[role="textbox"]').invoke("text", "nil is bad");
    });

    cy.get("[data-testid=compile-run-button]").click();
    cy.contains("Compilation failed").should("exist");
  });

  it("valid code compilation", () => {
    cy.compileCounterContract();

    cy.contains("Compilation successful", { timeout: 10000 }).should("exist");
  });
});

describe("examples", () => {
  it("a user can select an example", () => {
    cy.get("[data-testid=examples-dropdown-button]").click();
    cy.get("li").contains("Async call").click();

    cy.contains(
      "Async call arguments: destination address (dst), callback address (msg.sender)",
    ).should("exist");
  });
});

describe("logs", () => {
  it("a user can view logs", () => {
    cy.get("[data-testid=playground-logs]").should("exist");
  });

  it("logs can be cleared", () => {
    // we can not compile empty code so adding some code
    cy.compileCounterContract();

    cy.contains("Compilation successful", { timeout: 10000 }).should("exist");

    cy.get("[data-testid=clear-logs]").click();
    cy.contains("Compilation successful").should("not.exist");
  });
});
