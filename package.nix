{
  buildGoModule,
  callPackage,
  dwarfs,
  lib,
  makeWrapper,
  version ? "dev",
}:
(buildGoModule.override {go = callPackage ./go.nix {};}) {
  pname = "appherder";
  inherit version;
  src = ./.;
  vendorHash = "sha256-hrqRutMJw96V61AKoyt4Oqya8STeGYlF5phb6O35L8Q=";
  subPackages = ["cmd/appherder"];
  env.CGO_ENABLED = "0";
  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];
  nativeBuildInputs = [makeWrapper];
  postInstall = ''
    wrapProgram $out/bin/appherder --prefix PATH : ${lib.makeBinPath [dwarfs]}
  '';
  meta.mainProgram = "appherder";
}
