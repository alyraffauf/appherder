{
  buildGoModule,
  glib,
  gtk4,
  lib,
  libadwaita,
  pkg-config,
  wrapGAppsHook4,
}: let
  version = "dev";
in
  buildGoModule {
    pname = "appherder-gui";
    inherit version;
    src = ./.;
    vendorHash = "sha256-YoNtqb5dflJNCZBstAQxP458ktpUighC8uYuqFWjTyo=";
    subPackages = ["cmd/appherder-gui"];
    tags = ["gtk"];
    ldflags = ["-X main.version=${version}"];

    nativeBuildInputs = [
      pkg-config
      wrapGAppsHook4
    ];

    buildInputs = [
      glib
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
