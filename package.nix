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
    vendorHash = "sha256-hrqRutMJw96V61AKoyt4Oqya8STeGYlF5phb6O35L8Q=";
    subPackages = ["cmd/appherder"];
    ldflags = ["-X main.version=${version}"];
    nativeBuildInputs = [makeWrapper];
    postInstall = ''
      wrapProgram $out/bin/appherder --prefix PATH : ${lib.makeBinPath [dwarfs]}
    '';
  }
