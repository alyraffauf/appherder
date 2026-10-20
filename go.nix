{
  fetchurl,
  go_1_27,
  lib,
}:
# Keep certificate verification patched until stable nixpkgs catches up.
if lib.versionAtLeast go_1_27.version "1.27.3"
then go_1_27
else
  go_1_27.overrideAttrs (finalAttrs: _: {
    version = "1.27.3";
    src = fetchurl {
      url = "https://go.dev/dl/go${finalAttrs.version}.src.tar.gz";
      hash = "sha256-bndJA1zzYPBrJXXRYup535/ajvZxiJ5cN1+OUHd/Jrs=";
    };
  })
