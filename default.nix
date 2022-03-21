{ lib
, pkgs
}:
pkgs.buildGoModule rec {
  name = "bass";
  src = ./.;

  vendorSha256 = lib.fileContents ./nix/vendorSha256.txt;

  nativeBuildInputs = [ pkgs.upx ];

  buildPhase = ''
    make -j
  '';

  # don't run tests here; they're too complicated
  doCheck = false;

  installPhase = ''
    mkdir -p $out/bin
    make DESTDIR=$out/bin install
  '';
}
