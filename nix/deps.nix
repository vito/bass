{ pkgs
}:
pkgs.dockerTools.streamLayeredImage {
  name = "bass-deps";
  contents = with pkgs; [
    # for running scripts
    bashInteractive
    # start-stop-daemon, for hack/buildkit/start/stop
    dpkg
    # https (for fetching go mods, etc.)
    cacert
    # go building + testing
    go
    gcc
    gotestsum
    # runtime tests
    buildkit
    runc
    # lsp tests
    neovim
    # packing bass.*.(tgz|zip)
    gzip
    gnutar
    zip
    # git plumbing
    git
    # compressing shim binaries
    upx
    # for building in test image
    gnumake
    # bare necessitites (cp, find, which, etc)
    busybox
  ];
  config = {
    Env = [
      "PATH=/share/go/bin:/bin"
      "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
    ];
  };
}
