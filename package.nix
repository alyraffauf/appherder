{
  buildGoModule,
  lib,
}:
buildGoModule {
  pname = "appherder";
  version = "dev";
  src = ./.;
  vendorHash = "sha256-oXy9rgCRpuNHsqLW2sRylUamjPjOcHjC66noG4koLXk=";
  subPackages = ["src"];

  postInstall = ''
    mv "$out/bin/src" "$out/bin/appherder"
  '';
}
