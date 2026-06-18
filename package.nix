{
  buildGoModule,
  lib,
}: let
  version = "dev";
in
  buildGoModule {
    pname = "appherder";
    inherit version;
    src = ./.;
    vendorHash = "sha256-cnxgWLpc8l/dvJHgj1PkJrSYmsH88pCTISM+pf7Ulg4=";
    subPackages = ["cmd/appherder"];
    ldflags = ["-X main.version=${version}"];
  }
