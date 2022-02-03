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
  vendorSha256 = "sha256-iWNY2O9nzV43flAmKn+WtPfplrJGmqF4WjGaMnDkBiY=";

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
