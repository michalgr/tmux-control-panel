{
  description = "A terminal user interface control panel for managing tmux sessions and git worktrees";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, utils }:
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages.default = pkgs.callPackage ./default.nix {};

        apps.default = utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };

        devShells.default = import ./shell.nix { inherit pkgs; };
      }
    );
}
