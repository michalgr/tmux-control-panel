{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    go
    gopls
    gotools
    go-tools
    golangci-lint
    git
    tmux
  ];

  shellHook = ''
    echo "Welcome to the tmux-control-panel development environment!"
    echo "Go $(go version) is available, along with tmux and git."
  '';
}
