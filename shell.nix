{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = [
    pkgs.go_1_25
    pkgs.docker
    pkgs.docker-compose
    pkgs.git
    pkgs.mysql80
    pkgs.redis
  ];

  shellHook = ''
    echo "EMS Dev Environment Ready"
    echo "Go version: $(go version)"
  '';
}
