{ pkgs ? import <nixpkgs> {} }:

pkgs.buildGoModule {
  pname = "tmux-control-panel";
  version = "0.1.0";

  src = ./.;

  vendorHash = "sha256-TUbaUoqDZoQTkcOMtoE/FlAiqkWN+x49JeGkDguh2UU=";

  nativeBuildInputs = [ pkgs.makeWrapper ];
  nativeCheckInputs = [ pkgs.git pkgs.tmux ];

  postInstall = ''
    wrapProgram $out/bin/tmux-control-panel \
      --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.git pkgs.tmux ]}
  '';

  meta = with pkgs.lib; {
    description = "A terminal user interface control panel for managing tmux sessions and git worktrees";
    license = licenses.mit;
    mainProgram = "tmux-control-panel";
  };
}
