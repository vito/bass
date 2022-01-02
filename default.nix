{ lib
, buildGoModule
, makeWrapper
, buildkit
}:

buildGoModule rec {
  pname = "bass";
  version = "0.0.1-alpha";
  src = ./.;

  # get using ./hack/get-nix-vendorsha
  vendorSha256 = "sha256-iwWJxknJ6/LunUu3sbUB/1Q9d6EiErknmxh5F73xBW4=";

  nativeBuildInputs = [ makeWrapper ];

  ldflags = [
    "-X github.com/vito/bass.Version=${version}"
  ];

  postInstall = ''
    wrapProgram $out/bin/bass \
      --prefix PATH : ${lib.makeBinPath [ buildkit ]}
  '';

  subPackages = [ "cmd/bass" "cmd/bass-lsp" ];
}
