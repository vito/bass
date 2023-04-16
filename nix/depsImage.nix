{ pkgs
}:
let
  env-shim = pkgs.runCommand "env-shim" { } ''
    mkdir -p $out/usr/bin
    ln -s ${pkgs.coreutils}/bin/env $out/usr/bin/env
  '';
  stream = pkgs.dockerTools.streamLayeredImage {
    name = "bass-deps";
    contents = pkgs.callPackage ./deps.nix { } ++ (with pkgs; [
      # https (for fetching go mods, etc.)
      cacert
      # bare necessitites (cp, find, which, etc)
      busybox
      # /usr/bin/env compat
      env-shim
    ]);
    config = {
      Env = [
        "PATH=/share/go/bin:/bin"
        "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
      ];
    };
  };
in
pkgs.runCommand "save-archive" {} ''
  ${stream} > $out
''
