{ pkgs ? import <nixpkgs> { }
, format ? "oci"
}:
let
  image = pkgs.dockerTools.streamLayeredImage {
    name = "example";
    contents = [
      pkgs.coreutils
      pkgs.hello
      pkgs.git
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
  ${image} | gzip --fast | skopeo --insecure-policy copy --quiet docker-archive:/dev/stdin ${format}:$out:latest
''
