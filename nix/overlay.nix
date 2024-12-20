self: super: {
  pkgsStatic = super.pkgsStatic // {
    nodejs_22 = super.pkgsStatic.nodejs_22.overrideAttrs (oldAttrs: {
      doCheck = false;
    });
  };
}

