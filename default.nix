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
  vendorSha256 = "sha256-BRnuBTSO1QPW6M7S/p9jO9YJcB7ac7toMz3rBlkTiFk=";

  nativeBuildInputs = [ makeWrapper ];

  ldflags = [
    "-X github.com/vito/bass.Version=${version}"
  ];

  preBuild = ''
    make
  '';

  postInstall = ''
    wrapProgram $out/bin/bass \
      --prefix PATH : ${lib.makeBinPath [ buildkit ]}
  '';

  subPackages = [ "cmd/bass" ];
}
