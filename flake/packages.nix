{inputs, ...}: let
  appherderModule = import ./hm-module.nix inputs.self;
in {
  flake = {
    homeModules = {
      default = appherderModule;
      appherder = appherderModule;
    };

    # Keep the older output name for flakes that still use it.
    homeManagerModules = {
      default = appherderModule;
      appherder = appherderModule;
    };
  };

  perSystem = {
    pkgs,
    self',
    ...
  }: {
    packages =
      {
        default = self'.packages.appherder;
        appherder = pkgs.callPackage ../package.nix {};
      }
      // pkgs.lib.optionalAttrs pkgs.stdenv.hostPlatform.isLinux {
        appherder-gui = pkgs.callPackage ../package-gui.nix {};
      };
  };
}
