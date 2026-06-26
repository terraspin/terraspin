{ pkgs ? import <nixpkgs> { config.allowUnfree = true; } }:

pkgs.mkShell {
  name = "terraspin-dev";

  buildInputs = with pkgs; [
    go_1_24
    gnumake
    goreleaser
    golangci-lint
    gotools
    gofumpt
    terraform
    opentofu
    ollama
    git
    gh
    jq
    yq-go
    shellcheck
    cosign
    gotestsum
  ];

  shellHook = ''
    echo "🛠️  Terraspin development shell"
    echo "───────────────────────────────"
    echo "  Go    : $(go version)"
    echo "  Tofu  : $(tofu version 2>/dev/null | head -1 || echo 'not found')"
    echo "  TF    : $(terraform version 2>/dev/null | head -1)"
    echo ""
    export GOPATH="${builtins.getEnv "HOME"}/go"
    mkdir -p "$GOPATH/bin"
    export PATH="$GOPATH/bin:$PATH"
    export GOTOOLCHAIN=local
  '';

  GOFLAGS = "-mod=mod";
  CGO_ENABLED = "0";
  TERM = "xterm-256color";
}
