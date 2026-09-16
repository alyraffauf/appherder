self: {
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.programs.appherder;
  serviceCfg = config.services.appherder;
  toml = pkgs.formats.toml {};

  appherderConfig =
    cfg.settings
    // {
      appimages_dir = cfg.appImagesDirectory;
      bin_dir = cfg.binDirectory;
      max_saved_versions = cfg.maxSavedVersions;
    };

  configFile = toml.generate "appherder-config.toml" appherderConfig;
  executable = lib.getExe cfg.package;
  serviceWriteDirectories = [
    cfg.appImagesDirectory
    "${config.xdg.dataHome}/applications"
    cfg.binDirectory
  ];
in {
  options.programs.appherder = {
    enable = lib.mkEnableOption "AppHerder AppImage manager";

    package = lib.mkPackageOption self.packages.${pkgs.stdenv.hostPlatform.system} "appherder" {};

    appImagesDirectory = lib.mkOption {
      type = lib.types.str;
      default = "${config.home.homeDirectory}/AppImages";
      defaultText = lib.literalExpression ''"\${config.home.homeDirectory}/AppImages"'';
      description = "Directory that AppHerder manages as the AppImage source of truth.";
    };

    binDirectory = lib.mkOption {
      type = lib.types.str;
      default = "${config.home.homeDirectory}/.local/bin";
      defaultText = lib.literalExpression ''"\${config.home.homeDirectory}/.local/bin"'';
      description = "Directory where AppHerder creates command symlinks.";
    };

    maxSavedVersions = lib.mkOption {
      type = lib.types.ints.positive;
      default = 3;
      description = "Number of old AppImage versions to keep for rollback.";
    };

    settings = lib.mkOption {
      inherit (toml) type;
      default = {};
      description = ''
        Additional values for {file}`$XDG_CONFIG_HOME/appherder/config.toml`.
        Use this option to define source overrides. The typed directory and
        version options take precedence over values in this attribute set.
      '';
    };
  };

  options.services.appherder = {
    enable = lib.mkEnableOption "automatic AppHerder sync";

    upgrade = {
      enable = lib.mkEnableOption "automatic AppHerder upgrades";

      interval = lib.mkOption {
        type = lib.types.str;
        default = "24h";
        description = "systemd time span between automatic upgrade runs.";
      };
    };
  };

  config = lib.mkMerge [
    (lib.mkIf cfg.enable {
      assertions = [
        {
          assertion = pkgs.stdenv.hostPlatform.isLinux;
          message = "programs.appherder is supported only on Linux";
        }
      ];

      home.packages = [cfg.package];

      xdg.configFile."appherder/config.toml".source = configFile;

      home.activation.appherderDirectories = lib.hm.dag.entryAfter ["writeBoundary"] ''
        run mkdir -p $VERBOSE_ARG -- ${lib.escapeShellArgs serviceWriteDirectories}
      '';
    })

    (lib.mkIf (serviceCfg.enable || serviceCfg.upgrade.enable) {
      programs.appherder.enable = lib.mkDefault true;

      systemd.user.services =
        lib.optionalAttrs serviceCfg.enable {
          appherder-sync = {
            Unit = {
              Description = "Reconcile AppImages with installed applications";
              X-Restart-Triggers = [cfg.package configFile];
            };
            Service = {
              Type = "oneshot";
              ExecStart = "${executable} sync";
            };
          };
        }
        // lib.optionalAttrs serviceCfg.upgrade.enable {
          appherder-upgrade = {
            Unit = {
              Description = "Download and install available AppImage updates";
              X-Restart-Triggers = [cfg.package configFile];
            };
            Service = {
              Type = "oneshot";
              ExecStart = "${executable} upgrade";
            };
          };
        };

      systemd.user.paths = lib.optionalAttrs serviceCfg.enable {
        appherder-sync = {
          Unit.Description = "Watch AppImages for AppHerder sync";
          Path = {
            PathChanged = cfg.appImagesDirectory;
            Unit = "appherder-sync.service";
          };
          Install.WantedBy = ["default.target"];
        };
      };

      systemd.user.timers = lib.optionalAttrs serviceCfg.upgrade.enable {
        appherder-upgrade = {
          Unit.Description = "Check for AppImage updates";
          Timer = {
            OnBootSec = "5min";
            OnUnitActiveSec = serviceCfg.upgrade.interval;
            Persistent = true;
            Unit = "appherder-upgrade.service";
          };
          Install.WantedBy = ["timers.target"];
        };
      };
    })
  ];
}
