{
  description = "Supertonic TTS API Service";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs, ... }:
    let
      inherit (nixpkgs) lib;
      systems = [ "x86_64-linux" "aarch64-linux" ];
      onnxruntime-overlay = import ./onnx.nix;
      
      # Build helper that works in both flake and module contexts
      buildSupertonic = pkgs: pkgs.buildGoModule {
        pname = "go-supertonic";
        version = "0.1.0";
        src = ./.;  # Relative to flake root, works in pure evaluation
        vendorHash = "sha256-U/JLHsijXjXZkyglMnFYX+Ezu/K3vJCusfMl7uC2NM0=";
        nativeBuildInputs = with pkgs; [ makeWrapper ];
        postInstall = ''
          wrapProgram $out/bin/go-supertonic \
            --set ONNXRUNTIME_LIB_PATH ${pkgs.onnxruntime}/lib/libonnxruntime.so \
            --prefix LD_LIBRARY_PATH : ${pkgs.onnxruntime}/lib
        '';
      };
    in
    {
      packages = lib.genAttrs systems (system:
        {
          default = buildSupertonic (import nixpkgs {
            inherit system;
            overlays = [ onnxruntime-overlay ];
          });
        }
      );
      
      devShells = lib.genAttrs systems (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ onnxruntime-overlay ];
          };
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              bashInteractive git-lfs go gopls onnxruntime
            ] ++ (lib.optionals (pkgs ? gomod2nix) [ gomod2nix ]);
            
            shellHook = ''
              export ONNXRUNTIME_LIB_PATH="${pkgs.onnxruntime}/lib/libonnxruntime.so"
              echo "ONNX Runtime version: ${pkgs.onnxruntime.version}"
              ${lib.optionalString (!(pkgs ? gomod2nix)) ''
                echo "NOTE: gomod2nix unavailable. Update vendorHash manually."
              ''}
            '';
          };
        }
      );
    } // {
      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.supertonic;
          # Apply overlay locally without requiring users to add it
          pkgsWithOverlay = import pkgs.path {
            inherit (pkgs) system config;
            overlays = (pkgs.overlays or []) ++ [ onnxruntime-overlay ];
          };
        in
        with lib;
        {
          options.services.supertonic = {
            enable = mkEnableOption "Supertonic TTS API service";
            package = mkOption {
              type = types.package;
              default = buildSupertonic pkgsWithOverlay;
              description = "The supertonic package to use";
            };
            # ... rest of your options ...
            port = mkOption {
              type = types.port;
              default = 8880;
              description = "Port to listen on";
            };
            assetsDir = mkOption {
              type = types.path;
              example = "/var/lib/supertonic/assets";
              description = "Base directory containing ONNX models and voice styles";
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
                  
                  # Use onnxruntime from overlaid pkgs
                  Environment = [
                    "ONNXRUNTIME_LIB_PATH=${pkgsWithOverlay.onnxruntime}/lib/libonnxruntime.so"
                    "LD_LIBRARY_PATH=${pkgsWithOverlay.onnxruntime}/lib"
                  ];
                  
                  ExecStart = "${cfg.package}/bin/go-supertonic \
                    --port ${toString cfg.port} \
                    --assets-dir ${cfg.assetsDir} \
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
