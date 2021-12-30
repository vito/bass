{ lib, buildGoModule }:

buildGoModule rec {
  pname = "bass";
  version = "0.0.1-alpha";
  src = ./.;

  # get using ./hack/get-nix-vendorsha
  vendorSha256 = "sha256-YauCD97LRd649ag/EQ8VrNc90oXszk/brBcj33mvjrM=";

  ldflags = [
    "-X github.com/vito/bass.Version=${version}"
  ];

  subPackages = [ "cmd/bass" "cmd/bass-lsp" ];
}
