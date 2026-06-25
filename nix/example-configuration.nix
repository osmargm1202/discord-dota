# Ejemplo de cómo añadir el bot a tu /etc/nixos/configuration.nix
# Copia este bloque y ajusta las rutas

{ config, pkgs, ... }:

let
  # Apunta al repo clonado (o usa un flake input)
  dotaBotSrc = /home/osmarg/Code/discord-dota;
  dotaBotPkg = (import dotaBotSrc {}).packages.${pkgs.system}.default;
in {

  imports = [
    "${dotaBotSrc}/nix/module.nix"
  ];

  services.dota-discord-bot = {
    enable = true;
    package = dotaBotPkg;

    # Archivo con los secretos — NO va en git
    # Crea con: sudo install -m 600 -o root /dev/null /etc/dota-bot/env
    # Luego edita y llena las variables del .env.example
    environmentFile = "/etc/dota-bot/env";

    # Puerto para MinIO de este bot (ajusta si 9100 está ocupado)
    minioPort = 9100;
    minioConsolePort = 9101;

    # Directorio de datos de MinIO
    dataDir = "/var/lib/dota-minio";
  };
}

# =============================================================================
# PASOS PARA DESPLEGAR
# =============================================================================
#
# 1. Clonar el repo:
#    git clone https://github.com/osmargm1202/discord-dota.git /home/osmarg/Code/discord-dota
#
# 2. Crear archivo de secretos:
#    sudo mkdir -p /etc/dota-bot
#    sudo install -m 600 -o root -g root /dev/null /etc/dota-bot/env
#    sudo nano /etc/dota-bot/env
#
#    Contenido mínimo (ver .env.example para todas las opciones):
#    ---
#    DISCORD_TOKEN=tu_token
#    SERVER_ID=tu_server_id
#    STRATZ_TOKEN=tu_stratz_token
#    NOTIFICATION_CHANNEL_ID=tu_canal_id
#    RANKING_CHANNEL_ID=1519494823642398852
#    POSTGRES_PASSWORD=password_seguro
#    MINIO_ROOT_USER=minio_admin
#    MINIO_ROOT_PASSWORD=password_minio_seguro
#    MINIO_ACCESS_KEY=minio_admin
#    MINIO_SECRET_KEY=password_minio_seguro
#    MINIO_BUCKET=dota-rankings
#    MINIO_PUBLIC_URL=https://dota-s3.fifrex.com
#    BASE_YEAR=2026
#    BACKFILL_DELAY_MS=700
#    ---
#    Nota: POSTGRES_PASSWORD no se usa directamente (PostgreSQL en NixOS usa
#    peer auth por defecto para usuarios locales). Si quieres password auth
#    descomenta la sección pg_hba en el módulo.
#
# 3. Calcular el vendorHash del módulo Go:
#    cd /home/osmarg/Code/discord-dota
#    nix build .#  2>&1 | grep "got:" | awk '{print $2}'
#    Pegar ese hash en flake.nix → vendorHash = "sha256-..."
#
# 4. Desplegar:
#    sudo nixos-rebuild switch
#
# 5. Ver logs:
#    journalctl -u dota-discord-bot -f
#    journalctl -u dota-discord-minio -f
#
# 6. Rollback si algo falla:
#    sudo nixos-rebuild switch --rollback
#
# =============================================================================
# CONVIVENCIA CON OTROS PROYECTOS
# =============================================================================
#
# PostgreSQL: instancia compartida del sistema
#   - Este bot usa database "dotabot", user "dotabot"
#   - Otros proyectos usan sus propias databases
#   - Sin conflicto
#
# MinIO: instancia DEDICADA en /var/lib/dota-minio puerto 9100
#   - services.minio del sistema (si existe) sigue en su puerto (9000)
#   - Docker MinIO containers siguen en sus puertos
#   - Este bot tiene su propio proceso minio completamente separado
#   - Sin conflicto
#
# CloudFlare tunnel: apunta discord-dota-minio:9000 → ahora apunta a localhost:9100
#   (ajustar la config del tunnel en el servidor)
