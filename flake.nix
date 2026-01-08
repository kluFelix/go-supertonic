{
  description = "Supertonic TTS API Service";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    systems.url = "github:nix-systems/default";
    gomod2nix.url = "github:nix-community/gomod2nix";
  };

  outputs = { self, nixpkgs, flake-utils, gomod2nix, ... }:
    let
      onnxruntime-overlay = import ./onnx.nix;
    in
      flake-utils.lib.eachDefaultSystem (
        system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ onnxruntime-overlay ];
          };

          # Helper for gomod2nix
          gomod2nix-pkgs = gomod2nix.legacyPackages.${system};
        in
          {
          devShells.default = pkgs.mkShell {
            packages = with pkgs; [
              bashInteractive
              git-lfs
              go
              gopls
              onnxruntime
              gomod2nix-pkgs.gomod2nix
            ];

            shellHook = ''
            export ONNXRUNTIME_LIB_PATH="${pkgs.onnxruntime}/lib/libonnxruntime.so"
            echo "ONNX Runtime version: ${pkgs.onnxruntime.version}"
            echo "ONNX Runtime library: $ONNXRUNTIME_LIB_PATH"
            '';
          };

          # Building the app "the nix way"
          packages.default = pkgs.buildGoModule {
            pname = "go-supertonic";
            version = "0.1.0";
            src = ./.;

            vendorHash = "sha256-U/JLHsijXjXZkyglMnFYX+Ezu/K3vJCusfMl7uC2NM0=";

            nativeBuildInputs = with pkgs; [ makeWrapper ];

            postInstall = ''
            wrapProgram $out/bin/go-supertonic \
              --set ONNXRUNTIME_LIB_PATH ${pkgs.onnxruntime}/lib/libonnxruntime.so \
              --prefix LD_LIBRARY_PATH : ${pkgs.onnxruntime}/lib
            '';
          };
        }
      ) // {
      # NixOS module to run it as a service
      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.supertonic;
        in
          with lib;
        {
          options.services.supertonic = {
            enable = mkEnableOption "Supertonic TTS API service";

            package = mkOption {
              type = types.package;
              default = self.packages.${pkgs.system}.default;
              description = "The supertonic package to use";
            };

            port = mkOption {
              type = types.port;
              default = 8880;
              description = "Port to listen on";
            };

            assetsDir = mkOption {
              type = types.path;
              example = "/var/lib/supertonic/assets";
              description = "Directory containing ONNX models and voice styles";
            };

            openFirewall = mkOption {
              type = types.bool;
              default = false;
              description = "Open firewall port for external access";
            };

            user = mkOption {
              type = types.str;
              default = "supertonic";
              description = "User to run the service";
            };

            group = mkOption {
              type = types.str;
              default = "supertonic";
              description = "Group to run the service";
            };

            totalStep = mkOption {
              type = types.int;
              default = 5;
              description = "Number of denoising steps";
            };

            defaultSpeed = mkOption {
              type = types.float;
              default = 1.0;
              description = "Default speech speed";
            };
          };

          config = mkIf cfg.enable (mkMerge [
            {
              users.users.${cfg.user} = {
                isSystemUser = true;
                group = cfg.group;
                home = "/var/lib/supertonic";
                createHome = true;
              };
              users.groups.${cfg.group} = {};
            }

            {
              systemd.services.supertonic = {
                description = "Supertonic TTS API Service";
                after = [ "network.target" ];
                wantedBy = [ "multi-user.target" ];

                serviceConfig = {
                  Type = "simple";
                  User = cfg.user;
                  Group = cfg.group;

                  Environment = [
                    "ONNXRUNTIME_LIB_PATH=${pkgs.onnxruntime}/lib/libonnxruntime.so"
                    "LD_LIBRARY_PATH=${pkgs.onnxruntime}/lib"
                  ];

                  ExecStart = "${cfg.package}/bin/go-supertonic \
                    --port ${toString cfg.port} \
                    --onnx-dir ${cfg.assetsDir}/onnx \
                    --total-step ${toString cfg.totalStep} \
                    --default-speed ${toString cfg.defaultSpeed}";

                  Restart = "on-failure";
                  RestartSec = "10s";
                };
              };
            }

            (mkIf (cfg.openFirewall && config.networking.firewall.enable) {
              networking.firewall.allowedTCPPorts = [ cfg.port ];
            })
          ]);
        };

      overlays.default = onnxruntime-overlay;
    };
}
