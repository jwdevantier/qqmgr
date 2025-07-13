# SPDX-License-Identifier: CC0-1.0
# SPDX-FileCopyrightText: NONE
{
  description = "GoCode - Go-based alternative to Claude Code with Ollama integration";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-darwin" "aarch64-darwin" "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (system:
        let pkgs = nixpkgs.legacyPackages.${system}; in
        {
          default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go_1_23
          gopls
          golangci-lint
          gotools
          git
          ripgrep
          # For external tool development
          shellcheck
          jq
        ];

        shellHook = ''
          echo "🚀 Go Development Environment"
          echo "Go version: $(go version)"
          echo "Project: $(pwd)"
          echo ""
          echo "Available commands:"
          echo "  go run		- Run"
          echo "  go build		- Build"
          echo "  go test ./...	- Run tests"
          echo ""
        '';
          };
        });
    };
}
