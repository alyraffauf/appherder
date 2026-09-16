{inputs, ...}: {
  perSystem = {
    lib,
    pkgs,
    ...
  }: let
    makeConfiguration = module:
      inputs.home-manager.lib.homeManagerConfiguration {
        inherit pkgs;
        modules = [
          inputs.self.homeModules.default
          {
            home = {
              username = "appherder-test";
              homeDirectory = "/home/appherder-test";
              stateVersion = "26.05";
            };
          }
          module
        ];
      };

    programOnly = makeConfiguration {
      programs.appherder.enable = true;
    };

    syncOnly = makeConfiguration {
      services.appherder.enable = true;
    };

    upgradeOnly = makeConfiguration {
      services.appherder.upgrade.enable = true;
    };
  in {
    checks = lib.optionalAttrs pkgs.stdenv.hostPlatform.isLinux {
      home-manager-module = pkgs.runCommand "appherder-home-manager-module" {} ''
        test -f ${programOnly.activationPackage}/home-files/.config/appherder/config.toml
        test ! -e ${programOnly.activationPackage}/home-files/.config/systemd/user/appherder-sync.service
        test ! -e ${programOnly.activationPackage}/home-files/.config/systemd/user/appherder-upgrade.service

        test -f ${syncOnly.activationPackage}/home-files/.config/systemd/user/appherder-sync.service
        test -f ${syncOnly.activationPackage}/home-files/.config/systemd/user/appherder-sync.path
        test ! -e ${syncOnly.activationPackage}/home-files/.config/systemd/user/appherder-upgrade.timer

        test -f ${upgradeOnly.activationPackage}/home-files/.config/systemd/user/appherder-upgrade.service
        test -f ${upgradeOnly.activationPackage}/home-files/.config/systemd/user/appherder-upgrade.timer
        test ! -e ${upgradeOnly.activationPackage}/home-files/.config/systemd/user/appherder-sync.path

        touch $out
      '';
    };
  };
}
