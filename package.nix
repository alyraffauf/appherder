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
    vendorHash = "sha256-YoNtqb5dflJNCZBstAQxP458ktpUighC8uYuqFWjTyo=";
    subPackages = ["cmd/appherder"];
    ldflags = ["-X main.version=${version}"];
    nativeBuildInputs = [makeWrapper];
    postInstall = ''
      wrapProgram $out/bin/appherder --prefix PATH : ${lib.makeBinPath [dwarfs]}
    '';
  }
