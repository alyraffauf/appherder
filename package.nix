{
  buildGoModule,
  dwarfs,
  lib,
  makeWrapper,
}: let
  version = "dev";
in
  buildGoModule {
    pname = "appherder";
    inherit version;
    src = ./.;
    vendorHash = "sha256-DW+OYl2Lr7j4ZGOD/Cml2/2yuauX4EudLRaYH15YtAA=";
    subPackages = ["cmd/appherder"];
    ldflags = ["-X main.version=${version}"];
    nativeBuildInputs = [makeWrapper];
    postInstall = ''
      wrapProgram $out/bin/appherder --prefix PATH : ${lib.makeBinPath [dwarfs]}
    '';
  }
