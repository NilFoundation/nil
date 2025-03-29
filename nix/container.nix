{ pkgs, lib, config, ... }:
with lib;
let
  solc = (pkgs.callPackage ./solc.nix { });
  nil = (pkgs.callPackage ./nil.nix { solc = solc; });
  explorer = (pkgs.callPackage ./nilexplorer.nix { });
  devnetConfig = {
    nild_config_dir = "etc/nild";
    nild_credentials_dir = "etc/nild";
    nild_p2p_base_tcp_port = 30303;
    nil_wipe_on_update = true;
    nil_rpc_port = 8529;
    pprof_base_tcp_port = 6060;
    nShards = 5;
    nil_config = [
      {
        id = 0;
        shards = [ 0 1 ];
        splitShards = true;
        dhtBootstrapPeersIdx = [ 0 2 3 ];
      }
      {
        id = 1;
        shards = [ 0 2 ];
        splitShards = true;
        dhtBootstrapPeersIdx = [ 0 1 3 ];
      }
      {
        id = 2;
        shards = [ 0 3 ];
        splitShards = true;
        dhtBootstrapPeersIdx = [ 0 1 2 ];
      }
      {
        id = 3;
        shards = [ 0 4 ];
        splitShards = true;
        dhtBootstrapPeersIdx = [ 0 1 2 ];
      }
    ];
    nil_archive_config = [{
      id = 0;
      shards = [ 0 1 2 3 4 ];
      bootstrapPeersIdx = [ 0 1 2 3 ];
      dhtBootstrapPeersIdx = [ 0 1 2 3 ];
    }];
    nil_rpc_config = [{
      id = 0;
      dhtBootstrapPeersIdx = [ 0 1 2 3 ];
      archiveNodeIndices = [ 0 ];
    }];
  };

  format = pkgs.formats.yaml { };

  configFiles = pkgs.stdenv.mkDerivation {
    name = "devnet-configs";
    src = format.generate "devnet.yaml" devnetConfig;
    dontUnpack = true;
    buildInputs = [ nil ];
    buildPhase = ''
      mkdir etc
      nild gen-configs --basedir var/lib "$src"

      base="$(pwd)"
      find etc/nild/ -type f -name "*.yaml" -exec sed -i "s#$base/etc/nild#/etc/nild#g" {} +
      find etc/nild/ -type f -name "*.yaml" -exec sed -i "s#$base/var/lib#/var/lib#g" {} +
    '';
    installPhase = ''
      mkdir -p $out/etc
      cp -r etc/nild/* $out/etc
    '';
  };

  runtime_config = pkgs.writeText "runtime_config.toml" ''
    DOCUMENTATION_URL = "https://docs.nil.foundation/nil/intro"
    GITHUB_URL = "https://github.com/NilFoundation/nil-hardhat-example"
    API_URL = "/explorer-api"
    COMETA_SERVICE_API_URL = "https://api.devnet.nil.foundation/api"
    RPC_TELEGRAM_BOT = "https://t.me/NilDevnetTokenBot"
    RPC_API_URL = "/api"
    PLAYGROUND_DOCS_URL = "https://docs.nil.foundation"
    PLAYGROUND_NILJS_URL = "https://github.com/NilFoundation/nil.js"
    PLAYGROUND_MULTI_TOKEN_URL = "https://docs.nil.foundation/nil/getting-started/essentials/tokens"
    PLAYGROUND_SUPPORT_URL = "https://t.me/+PT-6HyWK_LBmMmIx"
    PLAYGROUND_FEEDBACK_URL = "https://form.typeform.com/to/pDEAcSqd"
    API_REQUESTS_ENABLE_BATCHING = "true"
    RECENT_PROJECTS_STORAGE_LIMIT = 5
  '';

in
{
  boot.isContainer = true;

  networking.firewall.allowedTCPPorts = [ 80 ];

  environment.systemPackages = [ nil configFiles pkgs.vim ];

  environment.etc."nild".source = "${configFiles}/etc";

  environment.etc."explorer_backend/runtime-config.toml".source =
    runtime_config;

  environment.etc."exporter/exporter.yaml".text = ''
    clickhouse-password: ""
    clickhouse-endpoint: 127.0.0.1:9000
    clickhouse-login: "default"
    clickhouse-database: "nil_database"
    api-endpoint: http://127.0.0.1:8529
  '';

  users.users.nil = {
    isSystemUser = true;
    group = "nil";
  };
  users.groups.nil = { };

  systemd.services = builtins.listToAttrs
    (map
      (cfg: {
        name = "nil-${toString cfg.id}";
        value = {
          description = "nil-${toString cfg.id} service";
          after = [ "network.target" ];
          wantedBy = [ "multi-user.target" ];
          serviceConfig = {
            ExecStart =
              "${nil}/bin/nild run -c /etc/nild/nil-${toString cfg.id}/nild.yaml";
            Restart = "always";
            User = "nil";
            Group = "nil";
            WorkingDirectory = "/var/lib/nil-${toString cfg.id}";
            StateDirectory = "nil-${toString cfg.id}";
            RuntimeDirectory = "nil-${toString cfg.id}";

          };
        };
      })
      devnetConfig.nil_config) //

  (builtins.listToAttrs (map
    (cfg: {
      name = "nil-rpc-${toString cfg.id}";
      value = {
        description = "nil-rpc-${toString cfg.id} service";
        after = [ "network.target" ];
        wantedBy = [ "multi-user.target" ];
        serviceConfig = {
          ExecStart = "${nil}/bin/nild rpc -c /etc/nild/nil-rpc-${
              toString cfg.id
            }/nild.yaml";
          Restart = "always";
          User = "nil";
          Group = "nil";
          WorkingDirectory = "/var/lib/nil-rpc-${toString cfg.id}";
          StateDirectory = "nil-rpc-${toString cfg.id}";
          RuntimeDirectory = "nil-rpc-${toString cfg.id}";

        };
      };
    })
    devnetConfig.nil_rpc_config)) //

  (builtins.listToAttrs (map
    (cfg: {
      name = "nil-archive-${toString cfg.id}";
      value = {
        description = "nil-archive-${toString cfg.id} service";
        after = [ "network.target" ];
        wantedBy = [ "multi-user.target" ];
        serviceConfig = {
          ExecStart = "${nil}/bin/nild archive -c /etc/nild/nil-archive-${
              toString cfg.id
            }/nild.yaml";
          Restart = "always";
          User = "nil";
          Group = "nil";
          WorkingDirectory = "/var/lib/nil-archive-${toString cfg.id}";
          StateDirectory = "nil-archive-${toString cfg.id}";
          RuntimeDirectory = "nil-archive-${toString cfg.id}";

        };
      };
    })
    devnetConfig.nil_archive_config)) //

  {
    exporter = {
      description = "exporter service";
      after = [ "network.target" "clickhouse.service" ];
      wantedBy = [ "multi-user.target" ];
      serviceConfig = {
        ExecStart =
          "${nil}/bin/exporter -c /etc/exporter/exporter.yaml --allow-db-clear";
        Restart = "always";
        User = "nil";
        Group = "nil";
        WorkingDirectory = "/var/lib/exporter";
        StateDirectory = "exporter";
        RuntimeDirectory = "exporter";
        ExecStartPre = ''
          ${pkgs.clickhouse}/bin/clickhouse-client --query "CREATE DATABASE IF NOT EXISTS nil_database"'';
      };
    };
  } // {
    explorer_backend = {
      description = "explorer_backend service";
      after = [ "network.target" "clickhouse.service" ];
      wantedBy = [ "multi-user.target" ];
      path = [ "${pkgs.nodejs}" "${pkgs.bash}" ];
      serviceConfig = {
        ExecStart =
          "${pkgs.nodejs}/bin/npm run start --prefix ${explorer}/explorer_backend";
        Restart = "always";
        User = "nil";
        Group = "nil";
        WorkingDirectory = "/var/lib/explorer_backend";
        StateDirectory = "explorer_backend";
        RuntimeDirectory = "explorer_backend";
        Environment = [
          "EXPLORER_DB=/var/lib/explorer_backend/explorer.db"
          "DB_URL=http://127.0.0.1:8123"
          "DB_USER=default"
          "DB_NAME=nil_database"
        ];
      };
    };
  };

  services.nginx = {
    enable = true;
    recommendedGzipSettings = true;
    recommendedOptimisation = true;
    recommendedProxySettings = true;
    recommendedTlsSettings = true;
    virtualHosts."default" = {
      locations = {
        "/api" = { proxyPass = "http://127.0.0.1:8529"; };
        "/explorer-api" = {
          proxyPass = "http://127.0.0.1:3000/api/";
          extraConfig = ''
            rewrite ^/explorer-api(/.*)$ /api$1 break;
          '';
        };
        "/runtime-config.toml" = { root = "/etc/explorer_backend/"; };
        "/" = {
          root = "${explorer}/explorer_frontend/dist";
          tryFiles = "$uri $uri/ /index.html";
        };
      };
      default = true;
    };
  };

  services.clickhouse = { enable = true; };
}
