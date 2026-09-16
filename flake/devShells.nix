_: {
  perSystem = {
    config,
    lib,
    pkgs,
    ...
  }: {
    devShells.default = pkgs.mkShell {
      packages =
        (with pkgs; [
          dwarfs
          glib
          go
          gobject-introspection
          gtk4
          just
          libadwaita
          pkg-config
        ])
        ++ lib.attrValues config.treefmt.build.programs;

      shellHook = ''
        export GOCACHE="''${XDG_CACHE_HOME:-$HOME/.cache}/appherder/go-build"
        mkdir -p "$GOCACHE"
      '';
    };
  };
}
