{ stdenv
, pkgs
}:
stdenv.mkDerivation rec {
  pname = "update_public_repo";
  version = "0.0.1";

  # Necessary build inputs: git and git-filter-repo
  propagatedBuildInputs = with pkgs; [
    git
    git-filter-repo
  ];
}
