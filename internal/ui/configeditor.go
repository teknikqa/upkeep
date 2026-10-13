package ui

import (
	"fmt"

	"github.com/pterm/pterm"

	"github.com/teknikqa/upkeep/internal/config"
)

// RunConfigEditor launches the interactive TUI config editor.
// Returns the edited config and whether the user chose to save.
func RunConfigEditor(cfg *config.Config) (*config.Config, bool, error) {
	if !IsTTY() {
		return nil, false, fmt.Errorf("interactive config editor requires a terminal (TTY)")
	}

	// Work on a copy so cancellation discards changes.
	edited := copyConfig(cfg)

	pterm.DefaultHeader.Println("upkeep — Configuration Editor")
	fmt.Println()

	for {
		choice, err := selectOption("Select a section to edit", []string{
			"General",
			"Providers",
			"Notifications",
			"Logging",
			"Save & Exit",
			"Exit without saving",
		})
		if err != nil {
			return nil, false, fmt.Errorf("menu selection: %w", err)
		}

		switch choice {
		case "General":
			if err := editGeneralSection(edited); err != nil {
				return nil, false, err
			}

		case "Providers":
			if err := editProvidersSection(edited); err != nil {
				return nil, false, err
			}

		case "Notifications":
			if err := editNotificationsSection(edited); err != nil {
				return nil, false, err
			}

		case "Logging":
			if err := editLoggingSection(edited); err != nil {
				return nil, false, err
			}

		case "Save & Exit":
			if err := config.Validate(edited); err != nil {
				pterm.Error.Printfln("Validation failed: %v", err)
				pterm.Info.Println("Please fix the issue and try again.")
				continue
			}
			return edited, true, nil

		case "Exit without saving":
			return nil, false, nil
		}
	}
}

// editGeneralSection edits top-level config fields.
func editGeneralSection(cfg *config.Config) error {
	for {
		choice, err := selectOption("General Settings", []string{
			formatMenuLabel("Parallelism", cfg.Parallelism),
			"Back",
		})
		if err != nil {
			return fmt.Errorf("general menu: %w", err)
		}

		if choice == "Back" {
			return nil
		}

		val, err := editInt("Parallelism", cfg.Parallelism, 1, 32)
		if err != nil {
			return err
		}
		cfg.Parallelism = val
	}
}

// editNotificationsSection edits notification settings.
func editNotificationsSection(cfg *config.Config) error {
	for {
		choice, err := selectOption("Notification Settings", []string{
			formatMenuLabel("Enabled", cfg.Notifications.Enabled),
			formatMenuLabel("Tool", cfg.Notifications.Tool),
			"Back",
		})
		if err != nil {
			return fmt.Errorf("notifications menu: %w", err)
		}

		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			val, err := editBool("Enable notifications", cfg.Notifications.Enabled)
			if err != nil {
				return err
			}
			cfg.Notifications.Enabled = val
		case startsWith(choice, "Tool"):
			val, err := editEnum("Notification tool", cfg.Notifications.Tool, []string{"terminal-notifier", "osascript"})
			if err != nil {
				return err
			}
			cfg.Notifications.Tool = val
		}
	}
}

// editLoggingSection edits logging settings.
func editLoggingSection(cfg *config.Config) error {
	for {
		choice, err := selectOption("Logging Settings", []string{
			formatMenuLabel("Dir", cfg.Logging.Dir),
			formatMenuLabel("Level", cfg.Logging.Level),
			"Back",
		})
		if err != nil {
			return fmt.Errorf("logging menu: %w", err)
		}

		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Dir"):
			val, err := editString("Log directory", cfg.Logging.Dir)
			if err != nil {
				return err
			}
			cfg.Logging.Dir = val
		case startsWith(choice, "Level"):
			val, err := editEnum("Log level", cfg.Logging.Level, []string{"debug", "info", "warn", "error"})
			if err != nil {
				return err
			}
			cfg.Logging.Level = val
		}
	}
}

// editProvidersSection shows the provider list and dispatches to per-provider editors.
func editProvidersSection(cfg *config.Config) error {
	p := &cfg.Providers
	for {
		options := []string{
			formatMenuLabel("Brew", p.Brew.Enabled),
			formatMenuLabel("Brew Cask", p.BrewCask.Enabled),
			formatMenuLabel("npm", p.Npm.Enabled),
			formatMenuLabel("pnpm", p.Pnpm.Enabled),
			formatMenuLabel("Yarn", p.Yarn.Enabled),
			formatMenuLabel("Bun", p.Bun.Enabled),
			formatMenuLabel("Composer", p.Composer.Enabled),
			formatMenuLabel("pip", p.Pip.Enabled),
			formatMenuLabel("Rust", p.Rust.Enabled),
			formatMenuLabel("uv", p.Uv.Enabled),
			formatMenuLabel("Editor Extensions", p.Editor.Enabled),
			formatMenuLabel("Oh My Zsh", p.Omz.Enabled),
			formatMenuLabel("Vim", p.Vim.Enabled),
			formatMenuLabel("Vagrant", p.Vagrant.Enabled),
			formatMenuLabel("Mac App Store", p.Mas.Enabled),
			formatMenuLabel("MacPorts", p.Macports.Enabled),
			"Back",
		}

		choice, err := selectOption("Select a provider to configure", options)
		if err != nil {
			return fmt.Errorf("providers menu: %w", err)
		}

		var editErr error
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Brew Cask"):
			editErr = editBrewCaskProvider(cfg)
		case startsWith(choice, "Brew"):
			editErr = editBrewProvider(cfg)
		case startsWith(choice, "npm"):
			editErr = editEnabledSkipProvider("npm", "Skip packages", &p.Npm.Enabled, &p.Npm.Skip)
		case startsWith(choice, "pnpm"):
			editErr = editEnabledOnlyProvider("pnpm", &p.Pnpm.Enabled)
		case startsWith(choice, "Yarn"):
			editErr = editEnabledOnlyProvider("Yarn", &p.Yarn.Enabled)
		case startsWith(choice, "Bun"):
			editErr = editEnabledOnlyProvider("Bun", &p.Bun.Enabled)
		case startsWith(choice, "Composer"):
			editErr = editEnabledOnlyProvider("Composer", &p.Composer.Enabled)
		case startsWith(choice, "pip"):
			editErr = editPipProvider(cfg)
		case startsWith(choice, "Rust"):
			editErr = editRustProvider(cfg)
		case startsWith(choice, "uv"):
			editErr = editUvProvider(cfg)
		case startsWith(choice, "Editor Extensions"):
			editErr = editEditorProvider(cfg)
		case startsWith(choice, "Oh My Zsh"):
			editErr = editEnabledOnlyProvider("Oh My Zsh", &p.Omz.Enabled)
		case startsWith(choice, "Vim"):
			editErr = editVimProvider(cfg)
		case startsWith(choice, "Vagrant"):
			editErr = editVagrantProvider(cfg)
		case startsWith(choice, "Mac App Store"):
			editErr = editEnabledSkipProvider("Mac App Store", "Skip apps", &p.Mas.Enabled, &p.Mas.Skip)
		case startsWith(choice, "MacPorts"):
			editErr = editEnabledSkipProvider("MacPorts", "Skip ports", &p.Macports.Enabled, &p.Macports.Skip)
		}
		if editErr != nil {
			return editErr
		}
	}
}

// --- Per-provider editors ---

// editEnabledOnlyProvider edits a provider whose only setting is Enabled.
func editEnabledOnlyProvider(name string, enabled *bool) error {
	for {
		choice, err := selectOption(name+" Settings", []string{
			formatMenuLabel("Enabled", *enabled),
			"Back",
		})
		if err != nil {
			return err
		}
		if choice == "Back" {
			return nil
		}
		v, err := editBool("Enable "+name, *enabled)
		if err != nil {
			return err
		}
		*enabled = v
	}
}

// editEnabledSkipProvider edits a provider with Enabled and Skip settings.
func editEnabledSkipProvider(name, skipPrompt string, enabled *bool, skip *[]string) error {
	for {
		choice, err := selectOption(name+" Settings", []string{
			formatMenuLabel("Enabled", *enabled),
			formatMenuLabel("Skip", *skip),
			"Back",
		})
		if err != nil {
			return err
		}
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			v, err := editBool("Enable "+name, *enabled)
			if err != nil {
				return err
			}
			*enabled = v
		case startsWith(choice, "Skip"):
			v, err := editStringSlice(skipPrompt, *skip)
			if err != nil {
				return err
			}
			*skip = v
		}
	}
}

func editUvProvider(cfg *config.Config) error {
	for {
		choice, err := selectOption("uv Settings", []string{
			formatMenuLabel("Enabled", cfg.Providers.Uv.Enabled),
			formatMenuLabel("Self Update", cfg.Providers.Uv.SelfUpdate),
			formatMenuLabel("Tools", cfg.Providers.Uv.Tool),
			"Back",
		})
		if err != nil {
			return err
		}
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			v, err := editBool("Enable uv", cfg.Providers.Uv.Enabled)
			if err != nil {
				return err
			}
			cfg.Providers.Uv.Enabled = v
		case startsWith(choice, "Self Update"):
			v, err := editBool("Run uv self update", cfg.Providers.Uv.SelfUpdate)
			if err != nil {
				return err
			}
			cfg.Providers.Uv.SelfUpdate = v
		case startsWith(choice, "Tools"):
			v, err := editBool("Upgrade uv tool packages", cfg.Providers.Uv.Tool)
			if err != nil {
				return err
			}
			cfg.Providers.Uv.Tool = v
		}
	}
}

func editBrewProvider(cfg *config.Config) error {
	for {
		choice, err := selectOption("Brew Settings", []string{
			formatMenuLabel("Enabled", cfg.Providers.Brew.Enabled),
			formatMenuLabel("Skip", cfg.Providers.Brew.Skip),
			formatMenuLabel("Post Hooks", cfg.Providers.Brew.PostHooks),
			"Back",
		})
		if err != nil {
			return err
		}
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			v, err := editBool("Enable Brew", cfg.Providers.Brew.Enabled)
			if err != nil {
				return err
			}
			cfg.Providers.Brew.Enabled = v
		case startsWith(choice, "Skip"):
			v, err := editStringSlice("Skip formulae", cfg.Providers.Brew.Skip)
			if err != nil {
				return err
			}
			cfg.Providers.Brew.Skip = v
		case startsWith(choice, "Post Hooks"):
			v, err := editStringSlice("Post hooks", cfg.Providers.Brew.PostHooks)
			if err != nil {
				return err
			}
			cfg.Providers.Brew.PostHooks = v
		}
	}
}

func editBrewCaskProvider(cfg *config.Config) error {
	for {
		choice, err := selectOption("Brew Cask Settings", []string{
			formatMenuLabel("Enabled", cfg.Providers.BrewCask.Enabled),
			formatMenuLabel("Greedy", cfg.Providers.BrewCask.Greedy),
			formatMenuLabel("Skip", cfg.Providers.BrewCask.Skip),
			formatMenuLabel("Auth Strategy", cfg.Providers.BrewCask.AuthStrategy),
			formatMenuLabel("Auth Overrides", cfg.Providers.BrewCask.AuthOverrides),
			formatMenuLabel("Rebuild Open With", cfg.Providers.BrewCask.RebuildOpenWith),
			"Back",
		})
		if err != nil {
			return err
		}
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			v, err := editBool("Enable Brew Cask", cfg.Providers.BrewCask.Enabled)
			if err != nil {
				return err
			}
			cfg.Providers.BrewCask.Enabled = v
		case startsWith(choice, "Greedy"):
			v, err := editBool("Greedy mode", cfg.Providers.BrewCask.Greedy)
			if err != nil {
				return err
			}
			cfg.Providers.BrewCask.Greedy = v
		case startsWith(choice, "Skip"):
			v, err := editStringSlice("Skip casks", cfg.Providers.BrewCask.Skip)
			if err != nil {
				return err
			}
			cfg.Providers.BrewCask.Skip = v
		case startsWith(choice, "Auth Strategy"):
			v, err := editEnum("Auth strategy", cfg.Providers.BrewCask.AuthStrategy, []string{"defer", "force-interactive", "skip"})
			if err != nil {
				return err
			}
			cfg.Providers.BrewCask.AuthStrategy = v
		case startsWith(choice, "Auth Overrides"):
			v, err := editMapStringBool("Auth overrides", cfg.Providers.BrewCask.AuthOverrides)
			if err != nil {
				return err
			}
			cfg.Providers.BrewCask.AuthOverrides = v
		case startsWith(choice, "Rebuild Open With"):
			v, err := editBool("Rebuild Open With", cfg.Providers.BrewCask.RebuildOpenWith)
			if err != nil {
				return err
			}
			cfg.Providers.BrewCask.RebuildOpenWith = v
		}
	}
}

func editPipProvider(cfg *config.Config) error {
	for {
		choice, err := selectOption("pip Settings", []string{
			formatMenuLabel("Enabled", cfg.Providers.Pip.Enabled),
			formatMenuLabel("Upgrade pip", cfg.Providers.Pip.UpgradePip),
			formatMenuLabel("Upgrade setuptools", cfg.Providers.Pip.UpgradeSetuptools),
			formatMenuLabel("pipx", cfg.Providers.Pip.Pipx),
			"Back",
		})
		if err != nil {
			return err
		}
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			v, err := editBool("Enable pip", cfg.Providers.Pip.Enabled)
			if err != nil {
				return err
			}
			cfg.Providers.Pip.Enabled = v
		case startsWith(choice, "Upgrade pip"):
			v, err := editBool("Upgrade pip itself", cfg.Providers.Pip.UpgradePip)
			if err != nil {
				return err
			}
			cfg.Providers.Pip.UpgradePip = v
		case startsWith(choice, "Upgrade setuptools"):
			v, err := editBool("Upgrade setuptools", cfg.Providers.Pip.UpgradeSetuptools)
			if err != nil {
				return err
			}
			cfg.Providers.Pip.UpgradeSetuptools = v
		case startsWith(choice, "pipx"):
			v, err := editBool("Update pipx packages", cfg.Providers.Pip.Pipx)
			if err != nil {
				return err
			}
			cfg.Providers.Pip.Pipx = v
		}
	}
}

func editRustProvider(cfg *config.Config) error {
	for {
		choice, err := selectOption("Rust Settings", []string{
			formatMenuLabel("Enabled", cfg.Providers.Rust.Enabled),
			formatMenuLabel("Rustup", cfg.Providers.Rust.Rustup),
			formatMenuLabel("Cargo install-update", cfg.Providers.Rust.CargoInstallUpdate),
			"Back",
		})
		if err != nil {
			return err
		}
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			v, err := editBool("Enable Rust", cfg.Providers.Rust.Enabled)
			if err != nil {
				return err
			}
			cfg.Providers.Rust.Enabled = v
		case startsWith(choice, "Rustup"):
			v, err := editBool("Update rustup", cfg.Providers.Rust.Rustup)
			if err != nil {
				return err
			}
			cfg.Providers.Rust.Rustup = v
		case startsWith(choice, "Cargo"):
			v, err := editBool("Update cargo-installed tools", cfg.Providers.Rust.CargoInstallUpdate)
			if err != nil {
				return err
			}
			cfg.Providers.Rust.CargoInstallUpdate = v
		}
	}
}

func editEditorProvider(cfg *config.Config) error {
	for {
		choice, err := selectOption("Editor Extensions Settings", []string{
			formatMenuLabel("Enabled", cfg.Providers.Editor.Enabled),
			formatMenuLabel("Editors", cfg.Providers.Editor.Editors),
			formatMenuLabel("Timeout", cfg.Providers.Editor.Timeout),
			"Back",
		})
		if err != nil {
			return err
		}
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			v, err := editBool("Enable editor extensions", cfg.Providers.Editor.Enabled)
			if err != nil {
				return err
			}
			cfg.Providers.Editor.Enabled = v
		case startsWith(choice, "Editors"):
			v, err := editStringSlice("Editor executables", cfg.Providers.Editor.Editors)
			if err != nil {
				return err
			}
			cfg.Providers.Editor.Editors = v
		case startsWith(choice, "Timeout"):
			v, err := editInt("Timeout (seconds)", cfg.Providers.Editor.Timeout, 1, 3600)
			if err != nil {
				return err
			}
			cfg.Providers.Editor.Timeout = v
		}
	}
}

func editVimProvider(cfg *config.Config) error {
	for {
		choice, err := selectOption("Vim Settings", []string{
			formatMenuLabel("Enabled", cfg.Providers.Vim.Enabled),
			formatMenuLabel("Update Script", cfg.Providers.Vim.UpdateScript),
			formatMenuLabel("Pathogen Dir", cfg.Providers.Vim.PathogenDir),
			formatMenuLabel("Bundles Dir", cfg.Providers.Vim.BundlesDir),
			"Back",
		})
		if err != nil {
			return err
		}
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			v, err := editBool("Enable Vim", cfg.Providers.Vim.Enabled)
			if err != nil {
				return err
			}
			cfg.Providers.Vim.Enabled = v
		case startsWith(choice, "Update Script"):
			v, err := editString("Vim update script", cfg.Providers.Vim.UpdateScript)
			if err != nil {
				return err
			}
			cfg.Providers.Vim.UpdateScript = v
		case startsWith(choice, "Pathogen Dir"):
			v, err := editString("Pathogen directory", cfg.Providers.Vim.PathogenDir)
			if err != nil {
				return err
			}
			cfg.Providers.Vim.PathogenDir = v
		case startsWith(choice, "Bundles Dir"):
			v, err := editString("Bundles directory", cfg.Providers.Vim.BundlesDir)
			if err != nil {
				return err
			}
			cfg.Providers.Vim.BundlesDir = v
		}
	}
}

func editVagrantProvider(cfg *config.Config) error {
	for {
		choice, err := selectOption("Vagrant Settings", []string{
			formatMenuLabel("Enabled", cfg.Providers.Vagrant.Enabled),
			formatMenuLabel("Notify", cfg.Providers.Vagrant.Notify),
			"Back",
		})
		if err != nil {
			return err
		}
		switch {
		case choice == "Back":
			return nil
		case startsWith(choice, "Enabled"):
			v, err := editBool("Enable Vagrant", cfg.Providers.Vagrant.Enabled)
			if err != nil {
				return err
			}
			cfg.Providers.Vagrant.Enabled = v
		case startsWith(choice, "Notify"):
			v, err := editBool("Send notifications", cfg.Providers.Vagrant.Notify)
			if err != nil {
				return err
			}
			cfg.Providers.Vagrant.Notify = v
		}
	}
}

// --- Helpers ---

// copyConfig creates a deep copy of a Config.
func copyConfig(cfg *config.Config) *config.Config {
	c := *cfg
	c.Providers.Brew.Skip = copyStringSlice(cfg.Providers.Brew.Skip)
	c.Providers.Brew.PostHooks = copyStringSlice(cfg.Providers.Brew.PostHooks)
	c.Providers.BrewCask.Skip = copyStringSlice(cfg.Providers.BrewCask.Skip)
	c.Providers.BrewCask.AuthOverrides = copyStringBoolMap(cfg.Providers.BrewCask.AuthOverrides)
	c.Providers.Npm.Skip = copyStringSlice(cfg.Providers.Npm.Skip)
	c.Providers.Mas.Skip = copyStringSlice(cfg.Providers.Mas.Skip)
	c.Providers.Macports.Skip = copyStringSlice(cfg.Providers.Macports.Skip)
	c.Providers.Editor.Editors = copyStringSlice(cfg.Providers.Editor.Editors)
	return &c
}

func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	c := make([]string, len(s))
	copy(c, s)
	return c
}

func copyStringBoolMap(m map[string]bool) map[string]bool {
	if m == nil {
		return nil
	}
	c := make(map[string]bool, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// startsWith checks if s starts with prefix. Used for matching menu items
// that include a dynamic suffix like "[enabled]".
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
