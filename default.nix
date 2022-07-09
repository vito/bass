{ lib
, pkgs
}:
pkgs.buildGo118Module rec {
  name = "bass";
  src = ./.;

  vendorSha256 = lib.fileContents ./nix/vendorSha256.txt;

  nativeBuildInputs = with pkgs; [
    upx
    # for sanity checking that upx exists
    #
    # not needed by nix, but needed by Makefile
    which
  ];

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
