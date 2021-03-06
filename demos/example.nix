{ pkgs ? import <nixpkgs> { } }:
let
  image = pkgs.dockerTools.streamLayeredImage {
    name = "example";
    contents = [
      pkgs.coreutils
      pkgs.hello
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
  ${image} | gzip --fast | skopeo --insecure-policy copy --quiet docker-archive:/dev/stdin oci-archive:$out:latest
''
