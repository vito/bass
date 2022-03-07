{ pkgs ? import <nixpkgs> { } }:
let
  image = pkgs.dockerTools.streamLayeredImage {
    name = "example";
    contents = with pkgs; [
      coreutils
      hello
      bash
    ];
    config = {
      Env = [ "FOO=1" ];
    };
  };
in
pkgs.runCommand "convert-to-oci"
{
  nativeBuildInputs = [ pkgs.skopeo ];
} ''
  skopeo --version
  ${image} | gzip --fast | skopeo --tmpdir $TMPDIR --insecure-policy copy --quiet docker-archive:/dev/stdin oci-archive:$out:latest
''
