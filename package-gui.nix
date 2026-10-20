{
  buildGoModule,
  callPackage,
  glib,
  gobject-introspection,
  gtk4,
  lib,
  libadwaita,
  pkg-config,
  wrapGAppsHook4,
}: let
  version = "dev";
in
  (buildGoModule.override {go = callPackage ./go.nix {};}) {
    pname = "appherder-gui";
    inherit version;
    src = ./.;
    vendorHash = "sha256-hrqRutMJw96V61AKoyt4Oqya8STeGYlF5phb6O35L8Q=";
    subPackages = ["cmd/appherder-gui"];
    tags = ["gtk"];
    ldflags = ["-X main.version=${version}"];

    nativeBuildInputs = [
      pkg-config
      wrapGAppsHook4
    ];

    buildInputs = [
      glib
      gobject-introspection
      gtk4
      libadwaita
    ];

    meta = {
      description = "GTK/libadwaita interface for AppHerder";
      license = lib.licenses.gpl3Only;
      mainProgram = "appherder-gui";
      platforms = lib.platforms.linux;
    };
  }
