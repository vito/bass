{ pkgs ? import <nixpkgs> { }
, image
}:
pkgs.runCommand "convert-to-oci"
{
  nativeBuildInputs = [ pkgs.skopeo ];
} ''
  ${image} | gzip --fast | skopeo --tmpdir $TMPDIR --insecure-policy copy --quiet docker-archive:/dev/stdin oci-archive:$out:latest
''
