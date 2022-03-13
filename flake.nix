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
          bass = pkgs.callPackage ./default.nix { };
        } // (pkgs.lib.optionalAttrs pkgs.stdenv.isLinux {
          # for passing to 'docker load'
          deps = pkgs.callPackage ./img/deps.nix { };
          # for using as thunk images
          depsArchive = pkgs.callPackage ./img/docker-to-oci.nix {
            image = pkgs.callPackage ./img/deps.nix { };
          };
        });

        defaultPackage = packages.bass;

        defaultApp = {
          type = "app";
          program = "${packages.bass}/bin/bass";
        };

        devShell = pkgs.mkShell {
          nativeBuildInputs = [
            pkgs.go
            pkgs.golangci-lint
            pkgs.gopls
          ];
        };
      });
}
