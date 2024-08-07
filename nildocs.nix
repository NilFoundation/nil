{ stdenv, yarn, fixup-yarn-lock, nodejs, nil, openssl, fetchYarnDeps}:

stdenv.mkDerivation rec {
  name = "nil.docs";
  pname = "nildocs";
  src = ./docs;
  buildInputs = [ nodejs yarn openssl ] ;

  offlineCache = fetchYarnDeps {
    yarnLock = "${src}/yarn.lock";
    hash = "sha256-owA8jaVy6D8oOe2ZvjrVPMbBYXf4R+98NLMhuWE8P1k=";
  };

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    nil
    yarn
    fixup-yarn-lock
  ];

  postPatch = ''
      export HOME=$NIX_BUILD_TOP/fake_home
      yarn config --offline set yarn-offline-mirror $offlineCache
      fixup-yarn-lock yarn.lock
      yarn install --offline --frozen-lockfile --ignore-scripts --no-progress --non-interactive
      patchShebangs node_modules/
    '';

  buildPhase = ''
    runHook preBuild

    export NILJS_SRC=${./packages/niljs}
    export OPENRPC_JSON=${nil}/share/doc/nil/openrpc.json
    export NODE_OPTIONS=--openssl-legacy-provider
    yarn --offline build

    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    mv build $out

    runHook postInstall
  '';

}
