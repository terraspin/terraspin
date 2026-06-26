{ pkgs ? import <nixpkgs> { config.allowUnfree = true; } }:

pkgs.mkShell {
  name = "terraspin-dev";

  buildInputs = with pkgs; [
    # Go toolchain — PRD §7.2: Go 1.22+
    go_1_24

    # Build tools
    gnumake
    goreleaser          # PRD §7.2: binary distribution
    golangci-lint       # PRD §7.2: linting
    gotools             # goimports, etc.
    gofumpt             # stricter gofmt

    # Terraform ecosystem — PRD §6.1 F1: support 1.0+
    terraform
    opentofu            # OpenTofu 1.6+ support

    # LLM local inference — PRD §6.1 F3: Ollama support
    ollama

    # Dev tooling
    git
    gh                  # GitHub CLI — PRD §8.1: PR comment workflow
    jq                  # JSON inspection for plan files
    yq-go               # YAML for .terraspin.yml configs
    shellcheck          # shell script linting

    # Release signing — PRD §7.2: cosign
    cosign

    # Testing — PRD §7.2: testify + golden files
    gotestsum           # better test output

    # MCP server — PRD §6.3 F10: MCP transport
    # (mcp-go is a Go lib, not an external tool — handled via go modules)
  ];

  shellHook = ''
    echo "🛠️  Terraspin development shell"
    echo "───────────────────────────────"
    echo "  Go    : $(go version)"
    echo "  Tofu  : $(tofu version 2>/dev/null | head -1 || echo 'not found')"
    echo "  TF    : $(terraform version 2>/dev/null | head -1)"
    echo ""
    echo "  Build: make"
    echo "  Test : go test ./..."
    echo "  Lint : golangci-lint run"
    echo ""

    # Set GOPATH if not set
    export GOPATH="${builtins.getEnv "HOME"}/go"
    mkdir -p "$GOPATH/bin"
    export PATH="$GOPATH/bin:$PATH"

    # Go 1.24 toolchain aware
    export GOTOOLCHAIN=local

    # Pre-commit check: .terraspin.yml example
    if [ ! -f .terraspin.yml ]; then
      echo "  ⚠  No .terraspin.yml found — run 'terraspin config init' after build"
      echo ""
    fi
  '';

  # Environment variables useful for development
  GOFLAGS = "-mod=mod";
  CGO_ENABLED = "0";  # static binary — PRD §7.2

  # Dev experience tweaks
  TERM = "xterm-256color";
}
