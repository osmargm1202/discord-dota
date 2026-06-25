# NixOS module para el bot de Dota 2
# Importar en configuration.nix:
#
#   imports = [ /path/to/discord-dota/nix/module.nix ];
#   services.dota-discord-bot.enable = true;
#   services.dota-discord-bot.environmentFile = "/etc/dota-bot/env";
#
# NO choca con otros proyectos:
#   - PostgreSQL: instancia compartida del sistema, base de datos "dotabot" aislada
#   - MinIO: instancia DEDICADA en puerto 9100 con datos en /var/lib/dota-minio
#            (separada de cualquier otro services.minio o minio Docker)

{ config, lib, pkgs, ... }:

let
  cfg = config.services.dota-discord-bot;
in {

  options.services.dota-discord-bot = {
    enable = lib.mkEnableOption "Dota 2 Discord Bot";

    package = lib.mkOption {
      type = lib.types.package;
      description = "Paquete del bot. Usa el del flake o uno propio.";
    };

    environmentFile = lib.mkOption {
      type = lib.types.path;
      example = "/etc/dota-bot/env";
      description = ''
        Archivo con variables de entorno (secretos).
        Mismo formato que .env.example pero sin comentarios.
        Permisos recomendados: chmod 600, chown root.
        Ver .env.example en el repo para todas las variables.
      '';
    };

    minioPort = lib.mkOption {
      type = lib.types.port;
      default = 9100;
      description = "Puerto para MinIO dedicado del bot. Cambia si hay conflicto.";
    };

    minioConsolePort = lib.mkOption {
      type = lib.types.port;
      default = 9101;
      description = "Puerto de la consola web de MinIO.";
    };

    dataDir = lib.mkOption {
      type = lib.types.str;
      default = "/var/lib/dota-minio";
      description = "Directorio de datos de MinIO dedicado para este bot.";
    };

    postgresDatabase = lib.mkOption {
      type = lib.types.str;
      default = "dotabot";
      description = "Nombre de la base de datos PostgreSQL.";
    };

    postgresUser = lib.mkOption {
      type = lib.types.str;
      default = "dotabot";
      description = "Usuario de PostgreSQL para el bot.";
    };
  };

  config = lib.mkIf cfg.enable {

    # -------------------------------------------------------------------------
    # PostgreSQL: instancia compartida del sistema, solo agrega DB+user
    # No toca configuraciones de otros proyectos
    # -------------------------------------------------------------------------
    services.postgresql = {
      enable = true;
      package = lib.mkDefault pkgs.postgresql_16;

      ensureDatabases = [ cfg.postgresDatabase ];
      ensureUsers = [{
        name = cfg.postgresUser;
        ensureDBOwnership = true;
      }];
    };

    # -------------------------------------------------------------------------
    # MinIO DEDICADO para este bot — NO usa services.minio del sistema
    # Puerto propio, directorio propio → cero conflictos con otros proyectos
    # -------------------------------------------------------------------------
    users.users.dota-minio = {
      isSystemUser = true;
      group = "dota-minio";
      home = cfg.dataDir;
      createHome = true;
    };
    users.groups.dota-minio = {};

    systemd.services.dota-discord-minio = {
      description = "MinIO para Dota Discord Bot (puerto ${toString cfg.minioPort})";
      after = [ "network.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig = {
        Type = "notify";
        NotifyAccess = "all";
        User = "dota-minio";
        Group = "dota-minio";
        ExecStart = "${pkgs.minio}/bin/minio server ${cfg.dataDir} --address :${toString cfg.minioPort} --console-address :${toString cfg.minioConsolePort}";
        Restart = "on-failure";
        RestartSec = "5s";

        # Aislamiento: este proceso no puede ver datos de otros MinIO
        PrivateTmp = true;
        ProtectHome = true;
        ProtectSystem = "strict";
        ReadWritePaths = [ cfg.dataDir ];
      };

      # Credenciales MinIO desde el mismo environmentFile del bot
      # (MINIO_ROOT_USER y MINIO_ROOT_PASSWORD)
      environmentFile = cfg.environmentFile;

      environment = {
        MINIO_VOLUMES = cfg.dataDir;
      };
    };

    # -------------------------------------------------------------------------
    # El bot
    # -------------------------------------------------------------------------
    users.users.dota-bot = {
      isSystemUser = true;
      group = "dota-bot";
      home = "/var/lib/dota-discord-bot";
      createHome = true;
    };
    users.groups.dota-bot = {};

    systemd.services.dota-discord-bot = {
      description = "Dota 2 Discord Bot";
      after = [
        "network.target"
        "postgresql.service"
        "dota-discord-minio.service"
      ];
      requires = [
        "postgresql.service"
        "dota-discord-minio.service"
      ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig = {
        Type = "simple";
        User = "dota-bot";
        Group = "dota-bot";
        WorkingDirectory = "/var/lib/dota-discord-bot";
        ExecStart = "${cfg.package}/bin/dota-discord-bot";
        Restart = "always";
        RestartSec = "5s";

        # Hardening
        PrivateTmp = true;
        ProtectHome = true;
        ProtectSystem = "strict";
        ReadWritePaths = [
          "/var/lib/dota-discord-bot"
        ];
      };

      environmentFile = cfg.environmentFile;

      # Overrides inyectados por el módulo — el environmentFile puede sobreescribir si necesita
      # PostgreSQL: peer auth local (sin password, socket Unix)
      # MinIO: apunta al proceso dedicado en su puerto
      environment = {
        POSTGRES_DSN = lib.mkDefault "postgres://${cfg.postgresUser}@/${cfg.postgresDatabase}?host=/run/postgresql&sslmode=disable";
        MINIO_ENDPOINT = lib.mkDefault "localhost:${toString cfg.minioPort}";
      };
    };

    # Firewall: exponer MinIO solo si se necesita acceso externo (CloudFlare tunnel)
    # networking.firewall.allowedTCPPorts = [ cfg.minioPort ];
    # (comentado por defecto; el tunnel de CloudFlare no necesita abrir el puerto)
  };
}
