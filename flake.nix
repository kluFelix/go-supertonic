{
  description = "A flake for go development with a specific onnx version";
  
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    systems.url = "github:nix-systems/default";
    flake-utils = {
      url = "github:numtide/flake-utils";
      inputs.systems.follows = "systems";
    };
    gomod2nix.url = "github:nix-community/gomod2nix";
  };

  outputs = { self, nixpkgs, flake-utils, gomod2nix, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        # Import our custom overlay
        onnxruntime-overlay = import ./onnx.nix;
        
        # Apply the overlay to nixpkgs
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ onnxruntime-overlay ];
        };
        
        gomod2nix-pkg = gomod2nix.legacyPackages.${system}.gomod2nix;
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            bashInteractive
            git-lfs

            go
            gopls
            onnxruntime   # 1.23.0 from our overlay
            gomod2nix-pkg # from nix-comunity/gomod2nix-pkg
          ];
          
          shellHook = ''
            export ONNXRUNTIME_LIB_PATH="${pkgs.onnxruntime}/lib/libonnxruntime.so"
            echo "ONNX Runtime version: ${pkgs.onnxruntime.version}"
            echo "ONNX Runtime library: $ONNXRUNTIME_LIB_PATH"
          '';
        };
      }
    );
}
