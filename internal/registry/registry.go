package registry

type UpdateKind string

const (
	KindNPM      UpdateKind = "npm"
	KindUV       UpdateKind = "uv"
	KindBun      UpdateKind = "bun"
	KindDeno     UpdateKind = "deno"
	KindComposer UpdateKind = "composer"
	KindRustup   UpdateKind = "rustup"
)

type UpdateSpec struct {
	Kind       UpdateKind
	LatestOnly bool
}

type Synthetic struct {
	DirEnv      string
	DirHints    []string
	VersionFile string
	VersionJSON bool
}

type Tool struct {
	Name           string
	Family         string
	Commands       []string
	VersionArgs    []string
	VersionPattern string
	Source         string
	Update         *UpdateSpec
	Synthetic      *Synthetic
	Hint           string
}

func All() []Tool {
	return []Tool{
		{Name: "node", Family: "Node", Commands: []string{"node"}, VersionArgs: []string{"--version"}, VersionPattern: `v(\d+\.\d+\.\d+)`, Source: "node", Hint: "Update through nvm/fnm/volta/mise or your distro; toolcheck never changes the Node runtime."},
		{Name: "npm", Family: "Node", Commands: []string{"npm"}, VersionArgs: []string{"--version"}, VersionPattern: `(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)`, Source: "npm:npm", Update: &UpdateSpec{Kind: KindNPM}},
		{Name: "pnpm", Family: "Node", Commands: []string{"pnpm"}, VersionArgs: []string{"--version"}, Source: "npm:pnpm", Hint: "Corepack manages the global shim; check project packageManager pins before changing it."},
		{Name: "yarn", Family: "Node", Commands: []string{"yarn"}, VersionArgs: []string{"--version"}, Source: "npm:yarn", Hint: "Corepack-managed; Yarn Classic and Berry are different majors."},
		{Name: "bun", Family: "Node", Commands: []string{"bun"}, VersionArgs: []string{"--version"}, Source: "npm:bun", Update: &UpdateSpec{Kind: KindBun, LatestOnly: true}},
		{Name: "deno", Family: "Node", Commands: []string{"deno"}, VersionArgs: []string{"--version"}, VersionPattern: `deno (\d+\.\d+\.\d+)`, Source: "deno", Update: &UpdateSpec{Kind: KindDeno}},
		{Name: "corepack", Family: "Node", Commands: []string{"corepack"}, VersionArgs: []string{"--version"}, Source: "npm:corepack", Hint: "Bundled with Node; update Node/corepack deliberately."},

		{Name: "nvm", Family: "Version managers", Source: "github:nvm-sh/nvm", Synthetic: &Synthetic{DirEnv: "NVM_DIR", DirHints: []string{"~/.nvm"}, VersionFile: "package.json", VersionJSON: true}, Hint: "Version manager; update it yourself."},
		{Name: "fnm", Family: "Version managers", Commands: []string{"fnm"}, VersionArgs: []string{"--version"}, VersionPattern: `fnm (\d+\.\d+\.\d+)`, Source: "github:Schniz/fnm", Hint: "Version manager; update it yourself."},
		{Name: "volta", Family: "Version managers", Commands: []string{"volta"}, VersionArgs: []string{"--version"}, VersionPattern: `volta (\d+\.\d+\.\d+)`, Source: "github:volta-cli/volta", Hint: "Version manager; update it yourself."},
		{Name: "asdf", Family: "Version managers", Commands: []string{"asdf"}, VersionArgs: []string{"--version"}, Source: "github:asdf-vm/asdf", Hint: "Version manager; update it yourself."},
		{Name: "mise", Family: "Version managers", Commands: []string{"mise"}, VersionArgs: []string{"--version"}, VersionPattern: `(\d{4}\.\d+\.\d+)`, Source: "github:jdx/mise", Hint: "Version manager; update it yourself."},

		{Name: "python3", Family: "Python", Commands: []string{"python3"}, VersionArgs: []string{"--version"}, VersionPattern: `Python (\d+\.\d+\.\d+)`, Source: "none", Hint: "apt-managed on Ubuntu; leave to Ubuntu."},
		{Name: "pip", Family: "Python", Commands: []string{"pip3", "pip"}, VersionArgs: []string{"--version"}, VersionPattern: `pip (\d+\.\d+(?:\.\d+)?)`, Source: "pypi:pip", Hint: "apt-managed here; update outside apt only if you installed pip yourself."},
		{Name: "pipx", Family: "Python", Commands: []string{"pipx"}, VersionArgs: []string{"--version"}, Source: "pypi:pipx", Hint: "apt-managed here; update outside apt only if you installed pipx yourself."},
		{Name: "uv", Family: "Python", Commands: []string{"uv"}, VersionArgs: []string{"--version"}, VersionPattern: `uv (\d+\.\d+\.\d+)`, Source: "pypi:uv", Update: &UpdateSpec{Kind: KindUV}},
		{Name: "poetry", Family: "Python", Commands: []string{"poetry"}, VersionArgs: []string{"--version"}, Source: "pypi:poetry", Hint: "Use poetry self update or the installer that added it; pipx installs are managed by pipx."},
		{Name: "pdm", Family: "Python", Commands: []string{"pdm"}, VersionArgs: []string{"--version"}, Source: "pypi:pdm", Hint: "Use pdm self update; pipx/uv installs are managed by their installer."},
		{Name: "pixi", Family: "Python", Commands: []string{"pixi"}, VersionArgs: []string{"--version"}, Source: "pypi:pixi", Hint: "Use pixi self-update."},
		{Name: "hatch", Family: "Python", Commands: []string{"hatch"}, VersionArgs: []string{"--version"}, Source: "pypi:hatch", Hint: "Use hatch self update or the installer that added it."},
		{Name: "conda", Family: "Python", Commands: []string{"conda"}, VersionArgs: []string{"--version"}, VersionPattern: `conda (\d+\.\d+\.\d+)`, Source: "anaconda:conda", Hint: "Update manually; environment changes can break projects."},
		{Name: "micromamba", Family: "Python", Commands: []string{"micromamba"}, VersionArgs: []string{"--version"}, Source: "github:mamba-org/micromamba-releases", Hint: "Use micromamba self-update."},
		{Name: "pyenv", Family: "Python", Commands: []string{"pyenv"}, VersionArgs: []string{"--version"}, VersionPattern: `pyenv (\d+\.\d+\.\d+)`, Source: "github:pyenv/pyenv", Hint: "Version manager; update it yourself."},

		{Name: "rustup", Family: "Rust", Commands: []string{"rustup"}, VersionArgs: []string{"--version"}, VersionPattern: `rustup (\d+\.\d+\.\d+)`, Source: "rustup", Update: &UpdateSpec{Kind: KindRustup, LatestOnly: true}},
		{Name: "cargo", Family: "Rust", Commands: []string{"cargo"}, VersionArgs: []string{"--version"}, VersionPattern: `cargo (\d+\.\d+\.\d+)`, Source: "rust", Hint: "apt-managed here; rustup users update with rustup."},
		{Name: "rustc", Family: "Rust", Commands: []string{"rustc"}, VersionArgs: []string{"--version"}, VersionPattern: `rustc (\d+\.\d+\.\d+)`, Source: "rust", Hint: "apt-managed here; rustup users update with rustup."},
		{Name: "cargo-binstall", Family: "Rust", Commands: []string{"cargo-binstall"}, VersionArgs: []string{"--version"}, Source: "github:cargo-bins/cargo-binstall", Hint: "Update with cargo install cargo-binstall --force."},

		{Name: "go", Family: "Go", Commands: []string{"go"}, VersionArgs: []string{"version"}, VersionPattern: `go(\d+\.\d+(?:\.\d+)?)`, Source: "go", Hint: "apt-managed here; manual installs update by replacing the Go tree."},
		{Name: "g", Family: "Version managers", Commands: []string{"g"}, VersionArgs: []string{"--version"}, Source: "github:voidint/g", Hint: "Go version manager; update it yourself."},
		{Name: "gvm", Family: "Version managers", Source: "github:moovweb/gvm", Synthetic: &Synthetic{DirEnv: "GVM_ROOT", DirHints: []string{"~/.gvm"}}, Hint: "Go version manager; update it yourself."},
		{Name: "goenv", Family: "Version managers", Commands: []string{"goenv"}, VersionArgs: []string{"--version"}, Source: "github:go-nv/goenv", Hint: "Go version manager; update it yourself."},

		{Name: "ruby", Family: "Ruby", Commands: []string{"ruby"}, VersionArgs: []string{"--version"}, VersionPattern: `ruby (\d+\.\d+\.\d+)`, Source: "none", Hint: "apt-managed here; leave to Ubuntu."},
		{Name: "gem", Family: "Ruby", Commands: []string{"gem"}, VersionArgs: []string{"--version"}, Source: "rubygems:rubygems-update", Hint: "apt-managed here; RubyGems self-update only if you installed it yourself."},
		{Name: "bundler", Family: "Ruby", Commands: []string{"bundle", "bundler"}, VersionArgs: []string{"--version"}, VersionPattern: `Bundler version (\d+\.\d+\.\d+)`, Source: "rubygems:bundler", Hint: "Use gem update bundler when Ruby is not apt-managed."},
		{Name: "rbenv", Family: "Version managers", Commands: []string{"rbenv"}, VersionArgs: []string{"--version"}, VersionPattern: `rbenv (\d+\.\d+\.\d+)`, Source: "github:rbenv/rbenv", Hint: "Version manager; update it yourself."},
		{Name: "rvm", Family: "Version managers", Source: "github:rvm/rvm", Synthetic: &Synthetic{DirEnv: "RVM_DIR", DirHints: []string{"~/.rvm"}, VersionFile: "VERSION"}, Hint: "Version manager; update it yourself."},

		{Name: "composer", Family: "PHP", Commands: []string{"composer"}, VersionArgs: []string{"--version"}, VersionPattern: `Composer version (\d+\.\d+\.\d+)`, Source: "composer", Update: &UpdateSpec{Kind: KindComposer}},

		{Name: "sdkman", Family: "Version managers", Source: "github:sdkman/sdkman-cli", Synthetic: &Synthetic{DirEnv: "SDKMAN_DIR", DirHints: []string{"~/.sdkman"}, VersionFile: "var/version"}, Hint: "Version manager; update it yourself."},
		{Name: "jenv", Family: "Version managers", Commands: []string{"jenv"}, VersionArgs: []string{"--version"}, Source: "github:jenv/jenv", Hint: "Version manager; update it yourself."},

		{Name: "dotnet", Family: ".NET", Commands: []string{"dotnet"}, VersionArgs: []string{"--version"}, Source: "dotnet", Hint: "Use your distro or the .NET install script."},
		{Name: "ollama", Family: "AI", Commands: []string{"ollama"}, VersionArgs: []string{"--version"}, VersionPattern: `ollama version (?:is )?(\d+\.\d+\.\d+)`, Source: "github:ollama/ollama", Hint: "Re-run the official installer to update."},

		{Name: "brew", Family: "Package managers", Commands: []string{"brew"}, VersionArgs: []string{"--version"}, VersionPattern: `Homebrew (\d+\.\d+\.\d+)`, Source: "github:Homebrew/brew", Hint: "Externally managed; use brew update/upgrade."},
		{Name: "nix", Family: "Package managers", Commands: []string{"nix"}, VersionArgs: []string{"--version"}, VersionPattern: `nix \(Nix\) (\d+\.\d+\.\d+)`, Source: "github:NixOS/nix", Hint: "Externally managed; use your Nix profile/channel workflow."},
		{Name: "snap", Family: "Package managers", Commands: []string{"snap"}, VersionArgs: []string{"--version"}, VersionPattern: `snap\s+(\d+\.\d+(?:\.\d+)?)`, Source: "none", Hint: "System-managed; snap refreshes itself."},
		{Name: "flatpak", Family: "Package managers", Commands: []string{"flatpak"}, VersionArgs: []string{"--version"}, VersionPattern: `Flatpak (\d+\.\d+\.\d+)`, Source: "none", Hint: "System-managed; leave to your distro."},
	}
}

func ByName(name string) (Tool, bool) {
	for _, tool := range All() {
		if tool.Name == name {
			return tool, true
		}
	}
	for _, tool := range All() {
		for _, command := range tool.Commands {
			if command == name {
				return tool, true
			}
		}
	}
	return Tool{}, false
}
