{
  description = "Claude Code conversation history search TUI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = inputs:
    inputs.flake-parts.lib.mkFlake {inherit inputs;} {
      systems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];

      perSystem = {pkgs, ...}: let
        claude-history = pkgs.buildGoModule {
          pname = "claude-history";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-Rgr/9AbX4NnCjQKBXSGYq230PvuLF+l7daZBxvfVSwY=";
          subPackages = ["cmd/claude-history"];
          nativeBuildInputs = [pkgs.pkg-config];
          buildInputs = [pkgs.arrow-cpp pkgs.duckdb];
          meta.mainProgram = "claude-history";
        };
      in {
        packages = {
          default = claude-history;
          inherit claude-history;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [go gopls gotools];
        };
      };
    };
}
