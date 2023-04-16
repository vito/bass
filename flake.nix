{
  description = "a low fidelity scripting language for building reproducible artifacts";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      supportedSystems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];
    in
    flake-utils.lib.eachSystem supportedSystems (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      rec {
        packages = {
          default = pkgs.callPackage ./default.nix { };
        } // (pkgs.lib.optionalAttrs pkgs.stdenv.isLinux {
          depsImage = pkgs.callPackage ./nix/depsImage.nix { };
        });

        apps = {
          default = {
            type = "app";
            program = "${packages.default}/bin/bass";
          };
        };

        devShells = (pkgs.lib.optionalAttrs pkgs.stdenv.isLinux {
          default = pkgs.mkShell {
            nativeBuildInputs = pkgs.callPackage ./nix/deps.nix { } ++ (with pkgs; [
              gopls
              gh
            ]);
          };
        });
      });
}
