{ lib
, buildGoModule
, upx
}:

buildGoModule rec {
  name = "bass";
  src = ./.;

  # get using ./hack/get-nix-vendorsha
  vendorSha256 = "sha256-BCH0z7epZa2DpQm4rstLdkF3DU8maneejl76PwV0Idw=";

  nativeBuildInputs = [ upx ];

  buildPhase = ''
    make -j
  '';

  installPhase = ''
    mkdir -p $out/bin
    make DESTDIR=$out/bin install
  '';

  subPackages = [ "cmd/bass" ];

  # don't run tests here
  doCheck = false;
}
