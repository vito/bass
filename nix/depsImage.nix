{ pkgs
}:
pkgs.dockerTools.streamLayeredImage {
  name = "bass-deps";
  contents = pkgs.callPackage ./deps.nix {} ++ (with pkgs; [
    # https (for fetching go mods, etc.)
    cacert
    # bare necessitites (cp, find, which, etc)
    busybox
  ]);
  config = {
    Env = [
      "PATH=/share/go/bin:/bin"
      "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
    ];
  };
}
