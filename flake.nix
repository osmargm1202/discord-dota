{
  description = "Dota 2 Discord Bot — flake con NixOS module incluido";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        lib = nixpkgs.lib;
      in {
        # Paquete Go del bot
        packages.default = pkgs.buildGoModule {
          pname = "dota-discord-bot";
          version = "2.0.0";
          src = ./.;

          # Para obtener el hash correcto:
          #   1. Pon vendorHash = lib.fakeHash;
          #   2. Corre: nix build .#
          #   3. El error te da: got: sha256-XXXX
          #   4. Reemplaza lib.fakeHash con "sha256-XXXX"
          vendorHash = "sha256-JYpN2MA3jeqe8WqXKIDbvFWrWRN5tD2Db3cKhXI0edo=";

          env.CGO_ENABLED = "0";
          ldflags = [ "-s" "-w" ];

          # Excluir archivos de test y docs del build context
          excludedPackages = [ "cmd" ];
        };

        # Shell de desarrollo — incluye todo para correr pruebas locales sin Docker
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_25
            postgresql_16   # postgres, initdb, pg_isready, createdb
            minio           # minio server
            curl            # para verificar MinIO
          ];
          shellHook = ''
            echo "Dev shell listo. Para probar sin Docker:"
            echo "  nix build .#"
            echo "  ./nix/run-local-test.sh"
          '';
        };
      }
    ) // {
      # NixOS module para importar desde configuration.nix
      nixosModules.default = import ./nix/module.nix;
    };
}
