{
  buildGoModule,
  lib,
}:
buildGoModule {
  pname = "appherder";
  version = "dev";
  src = ./.;
  vendorHash = "sha256-cnxgWLpc8l/dvJHgj1PkJrSYmsH88pCTISM+pf7Ulg4=";
  subPackages = ["cmd/appherder"];
}
