{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = [
    pkgs.go_1_25
    pkgs.git
    pkgs.redis
    pkgs.mysql80
    pkgs.docker
    pkgs.docker-compose
  ];

  shellHook = ''
    echo "EMS Dev Environment Ready"

  '';
}
