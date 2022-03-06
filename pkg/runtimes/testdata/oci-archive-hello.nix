{ pkgs ? import <nixpkgs> { } }:
let
  image = pkgs.dockerTools.streamLayeredImage {
    name = "example";
    contents = [
      pkgs.bash
      pkgs.hello
    ];
    config = {
      Env = [ "GREETING=Hello, Bass!" ];
    };
  };
in
pkgs.runCommand "convert-to-oci"
{
  nativeBuildInputs = [ pkgs.skopeo ];
} ''
  ${image} | gzip --fast | skopeo --insecure-policy copy --quiet docker-archive:/dev/stdin oci-archive:$out:latest
''
