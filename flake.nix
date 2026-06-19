{
  description = "A herder for your Appimages";

  inputs.nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0";

  outputs = {self, ...} @ inputs: let
    inherit (inputs.nixpkgs) lib;

    supportedSystems = [
      "x86_64-linux"
      "aarch64-linux"
      "aarch64-darwin"
    ];

    forEachSupportedSystem = f:
      lib.genAttrs supportedSystems (
        system:
          f {
            inherit system;
            pkgs = import inputs.nixpkgs {
              inherit system;
              config.allowUnfree = true;
            };
          }
      );
  in {
    devShells = forEachSupportedSystem (
      {
        pkgs,
        system,
      }: {
        default = pkgs.mkShellNoCC {
          packages = with pkgs; [
            go
            dwarfs
            pkg-config
            glib
            gobject-introspection
            gtk4
            libadwaita
            self.formatter.${system}
          ];

          shellHook = ''
            export GOCACHE="''${XDG_CACHE_HOME:-$HOME/.cache}/appherder/go-build"
            mkdir -p "$GOCACHE"
          '';
        };
      }
    );

    formatter = forEachSupportedSystem ({pkgs, ...}: pkgs.alejandra);

    packages = forEachSupportedSystem (
      {pkgs, ...}: let
        appherder = pkgs.callPackage ./package.nix {};
      in
        {
          default = appherder;
          inherit appherder;
        }
        // lib.optionalAttrs pkgs.stdenv.isLinux {
          appherder-gui = pkgs.callPackage ./package-gui.nix {};
        }
    );
  };
}
