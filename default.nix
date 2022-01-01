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
  vendorSha256 = "sha256-qz80/KMWJllnjwoqE/6RO3afV/9hojWtXbMpXXWZM0I=";

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
