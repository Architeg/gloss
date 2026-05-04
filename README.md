<div align="center">
  <h1>Gloss</h1>
  <p><em>A local-first command glossary for your terminal.</em></p>
</div><br/>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white"/>
  <img alt="Platform" src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-2f855a"/>
  <img alt="Version" src="https://img.shields.io/badge/version-v0.1.0-111827"/>
  <img alt="License" src="https://img.shields.io/badge/license-MIT-2563eb"/>
  <a href="https://github.com/Architeg/gloss/commits/main">
    <img alt="Commit activity" src="https://img.shields.io/github/commit-activity/m/Architeg/gloss?label=commits"/>
  </a>
  <a href="https://github.com/Architeg/gloss/stargazers">
    <img alt="GitHub stars" src="https://img.shields.io/github/stars/Architeg/gloss?style=flat&label=stars"/>
  </a>
  <a href="https://worksfine.dev">
    <img alt="WorksFine.dev" src="https://img.shields.io/badge/WorksFine.dev-more%20apps%20%26%20tools-111827"/>
  </a>
  <a href="#-support-gloss">
    <img alt="Support Gloss" src="https://img.shields.io/badge/%E2%AD%90%20Support-Gloss-2563eb"/>
  </a>
</p>

<p align="center">
  <a href="https://www.uneed.best/tool/gloss">
    <img src="https://www.uneed.best/EMBED1B.png" alt="Published on Uneed" height="44" />
  </a>
  &nbsp;
  <a href="https://twelve.tools">
    <img src="https://twelve.tools/badge0-dark.svg" alt="Featured on Twelve Tools" height="38" />
  </a>
  &nbsp;
  <a href="https://www.producthunt.com/products/gloss?embed=true&utm_source=badge-featured&utm_medium=badge&utm_campaign=badge-gloss">
    <img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1138925&theme=dark&t=1777916961103" alt="Gloss - A command glossary for your terminal | Product Hunt" height="40" />
  </a>
  &nbsp;
  <a href="https://wired.business">
    <img src="https://wired.business/badge0-dark.svg" alt="Featured on Wired Business" height="38" />
  </a>
</p>

Gloss keeps reusable shell commands organized, searchable, and ready when you need them. Save commands, find them fast, and sync aliases safely on macOS/zsh and Linux/bash setups.

It is small, local-first, keyboard-first, and terminal-native.

<p align="center">
  <img src="assets/gloss-logo-dark-h2-home-screen.png" alt="Gloss logo" width="840"/>
</p>

## ✅ Features

### Command glossary

- Save command entries with descriptions and tags
- Browse entries in a clean terminal UI
- Search and filter in the TUI
- Add, edit, and delete entries

### Scan and import

- Scan your detected shell config and configured scan paths
- Supports zsh defaults (`~/.zshrc`) and bash defaults (`~/.bashrc`, `~/.bash_aliases`)
- Detect aliases
- Detect simple shell functions
- Detect executable files in configured directories
- Import selected suggestions into the glossary

### Managed aliases

- Add aliases directly from Gloss
- Preview and sync only the managed block into your configured shell file
- Uses `~/.zshrc` for zsh and `~/.bashrc` for bash by default
- Delete managed aliases cleanly
- Avoid rewriting the shell file when nothing changed

### Safety

- Backups are created only when sync actually changes an existing shell file
- Old Gloss-created backups are pruned automatically
- Alias add does not auto-sync
- Managed aliases stay inside a dedicated block

### CLI + TUI

- Use the interactive TUI for browsing, editing, and importing
- Use direct CLI commands for quick add/list/scan/edit/delete/version workflows

## ⚠️ Platform support

- **Officially supported:** macOS with zsh
- **Officially supported:** Linux with bash
- ❌ **Not officially supported yet:** Windows

Gloss detects your shell when creating its first config:

- zsh → `~/.zshrc`
- bash → `~/.bashrc`, and scans `~/.bash_aliases` too

Existing config is never overwritten automatically. Edit `~/.config/gloss/config.yaml` if you want to change shell or scan paths.

## 💾 Installation

### 🔽 Option 1 — Install script

```bash
curl -fsSL https://raw.githubusercontent.com/Architeg/gloss/main/scripts/install.sh | bash
```
By default, the script installs Gloss to `~/.local/bin/gloss`.

> [!NOTE]
> If ~/.local/bin is not in your PATH, the installer will print the exact commands to add it to your shell config.

Install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/Architeg/gloss/main/scripts/install.sh -o /tmp/gloss-install.sh
VERSION=v0.1.0 bash /tmp/gloss-install.sh
```

After installation:

```bash
gloss version
```

### 🔽 Option 2 — Homebrew

```bash
brew install Architeg/tap/gloss
```

Then:

```bash
gloss version
```

> [!NOTE]
> On some Homebrew setups, install may try to use a non-API path and behave unexpectedly.

Check this first:

```bash
echo "$HOMEBREW_NO_INSTALL_FROM_API"
```

If it returns `1`, unset it:

```bash
unset HOMEBREW_NO_INSTALL_FROM_API
```

Then retry:

```bash
brew install Architeg/tap/gloss
```

You can also skip auto-update during install:

```bash
HOMEBREW_NO_AUTO_UPDATE=1 brew install Architeg/tap/gloss
```

### 🔽 Option 3 — Manual install from GitHub Releases

Download the correct asset for your platform from the Releases page, then install manually.

Example for macOS Apple Silicon:

```bash
unzip gloss-darwin-arm64.zip
chmod +x gloss-darwin-arm64
sudo mv gloss-darwin-arm64 /usr/local/bin/gloss
gloss version
```

## 🗑️ Uninstall

If you installed Gloss with the install script, remove the binary:

```bash
rm -f "$HOME/.local/bin/gloss"
```
If you installed it system-wide:

```bash
sudo rm -f /usr/local/bin/gloss
```

If you installed with Homebrew:

```bash
brew uninstall gloss
```

Optional: remove local Gloss data and config:
```bash
rm -rf "$HOME/.config/gloss"
```
Optional: remove the managed alias block from your shell config manually from:

- `~/.zshrc` or
- `~/.bashrc`

```zsh
# >>> gloss aliases >>>
# ...
# <<< gloss aliases <<<
```
If the install script added Gloss to your PATH, you can also remove this block from your shell config:

```bash
# --- Path to Gloss ---
export PATH="$HOME/.local/bin:$PATH"
```

## 🚀 Quick start

Launch the TUI:

```bash
gloss
```

Or use direct CLI commands:

```bash
gloss help
gloss version
gloss list
gloss scan
gloss add
gloss edit <command>
gloss delete <command>
gloss alias add
gloss alias sync
gloss alias delete <name>
```

## CLI commands

### Version

```bash
gloss version
gloss --version
gloss -v
```

### Help

```bash
gloss help
```

### Add an entry

```bash
gloss add
```

Prompts for:

- command
- description
- tags

### List entries

```bash
gloss list
```

Filter by tag:

```bash
gloss list --tag git
```

### Scan sources

```bash
gloss scan
```

CLI scan is print-only.

Use the TUI **Scan** screen to select and import suggestions interactively.

### Edit an entry

```bash
gloss edit <command>
```

### Delete an entry

```bash
gloss delete <command>
```

### Managed aliases

Add a managed alias:

```bash
gloss alias add
```

Delete a managed alias:

```bash
gloss alias delete <name>
```

Sync managed aliases into your shell file:

```bash
gloss alias sync
```

## 🧩 TUI overview

Run:

```bash
gloss
```

Main sections:

- **Commands**
- **Add**
- **Scan**
- **Aliases**
- **Settings**
- **Readme**

The Home screen also includes support links.

<p align="center">
  <img src="assets/home-screen.png" alt="Gloss commands" width="840"/>
</p>

### Navigation

Gloss is keyboard-first.

Common keys:

- `↑` / `↓` — move
- `←` / `→` — switch support links on Home when that row is focused
- `Enter` — open / select / confirm
- `Esc` — go back
- `q` — quit

Gloss also supports Vim-style navigation in some places where applicable.


## 1. Commands screen

The **Commands** screen is the main glossary browser.

You can:

- browse saved entries grouped by tag
- open entry details
- add new entries
- edit existing entries
- delete entries
- search by command/description
- filter by tag

Entries without tags are shown under **Untagged**.

```bash

───────────────────────────── Commands ─────────────────────────────                          

Search:   > substring in command or description                                              
Tag:      > exact tag                                                                        


› Category: Git
───────────────────────

  gs                    git status
  ga                    git add .
  gc                    git commit -m
  gp                    git push                                                     


› Category: Tools
───────────────────────

  nano                  Open nano editor
  serve                 Start a local static file server
  updatebrew            brew update && brew upgrade                                               


› Category: Network
───────────────────────

  headers               curl -I
  pingg                 ping github.com
  myip                  curl ifconfig.me
  dns                   dig
  speed                 networkQuality



/ Search │ F Filter │ E Edit │ D Delete │ A Add │ ↑↓ Move │ Enter Open │ Esc Back │ Q Quit  
```

## 2. Add entry

The **Add** screen lets you create a new glossary entry directly from the TUI.

Each entry includes:

- command
- description
- tags

Tags are comma-separated, for example:

```text
git, shell, docker
```

After saving, the entry appears in the **Commands** screen under its tag group. If no tags are added, the entry appears under **Untagged**.

```bash

──────────────────────────── Add entry ─────────────────────────────                          

Command                                                                                       
> command                                                                                     

Description                                                                                   
> description                                                                                 

Tags                                                                                          
> tags (comma-separated)                                                                      



Esc Cancel │ Tab Field │ ^S Save │ Q Quit                       
```

## 3. Scan and import

Use the **Scan** screen in the TUI for the full workflow.

Gloss can detect:

- aliases from your configured shell file
- zsh aliases from `~/.zshrc`
- bash aliases from `~/.bashrc` and `~/.bash_aliases`
- aliases/functions from configured scan files
- executable files from configured scan directories

### Scan behavior

- suggestions are selected by default
- use `Space` to toggle items
- imported suggestions disappear after import
- remaining suggestions stay visible
- existing commands already in the glossary are skipped

### Why imported scan entries are uncategorized

Imported items are intentionally added without tags by default.

This keeps bulk import fast and avoids a prompt loop when scanning larger configs. You can tag them later if needed.

```bash

─────────────────────────────── Scan ───────────────────────────────                          

Sources                                                                                       
/Users/yourname/.zshrc                                                               

12 importable — 3 already in glossary                                                         

  [x] gs                  alias       git status
  [x] ga                  alias       git add .
  [x] gp                  alias       git push
  [x] gl                  alias       git pull --rebase
› [ ] ll                  alias       "ls -lah"
  [x] hide                alias       chflags hidden
  [x] nohide              alias       chflags nohidden

  [x] mkcd                function    shell function
  [x] serve               function    python3 -m http.server 8000
  [ ] precmd              function    shell function
  [ ] preexec             function    shell function

  [x] deploy              script      ./scripts/  deploy.sh                                                                                



↑↓ Move │ Space Toggle │ Enter Import │ R Rescan │ A All │ C Clear │ Esc Back │ Q Quit
```

## 4. Managed aliases

Gloss treats managed aliases as normal glossary entries with extra sync behavior.

```bash

───────────────────────────── Aliases ──────────────────────────────                          

Shell file                                                                                    
/Users/yourname/.zshrc                                                               

› Add Alias               Store in Gloss; sync separately to shell                            
  View Managed Aliases    Entries with managed alias flag                                     
  Preview Sync Block      Exact block written on sync                                         
  Sync to shell file      Backup if file exists, then write block                             




↑↓ Move │ Enter Open │ Esc Back │ Q Quit
```

### Add a managed alias

In the TUI:

1. Open **Aliases**
2. Choose **Add managed alias**

Or via CLI:

```bash
gloss alias add
```

This stores the alias in Gloss but does **not** immediately write to your shell file.

### Preview sync block

Before syncing, Gloss can show the exact block it will write:

```zsh
# >>> gloss aliases >>>
alias gs="git status"
alias ll="ls -lah"
# <<< gloss aliases <<<
```

### Sync behavior

When you sync:

```bash
gloss alias sync
```

Gloss will:

1. Build the managed alias block
2. Replace the existing Gloss-managed block if it exists
3. Append the block if it does not exist
4. Leave unrelated shell file content untouched

### No-op sync

If the generated block matches what is already in the shell file:

- Gloss does **not** rewrite the file
- Gloss does **not** create a backup
- Gloss shows an “already up to date” style message

### Delete a managed alias

Delete it from the TUI managed aliases list or via CLI:

```bash
gloss alias delete gs
```

Then sync again, and it will disappear from the managed block in your configured shell file.

## Safety and backups

Gloss is conservative by design.

### When backups are created

Backups are created **only** when:

- the shell file already exists
- sync is actually going to modify it

### When backups are not created

No backup is created when:

- there is no shell file yet and Gloss creates it for the first time
- sync detects there is no change to write

### Backup naming

Gloss uses timestamped backups, for example:

```bash
~/.zshrc.gloss.bak-20260423-223500
~/.bashrc.gloss.bak-20260423-223500
```

Old Gloss-created backups are pruned automatically to keep only a small recent set.

## 5. Settings

For v1, Settings is intentionally minimal and read-only.

It shows:

- shell file path
- storage path
- scan paths
- config file path

If needed, you can edit config manually.

## Configuration

Gloss stores config and data under:

```bash
~/.config/gloss/
```

Typical files:

```bash
~/.config/gloss/config.yaml
~/.config/gloss/gloss.db
```

Example config:

Example config for macOS/zsh:

```yaml
shell_file: /Users/yourname/.zshrc
storage_path: /Users/yourname/.config/gloss
scan_paths:
  - /Users/yourname/.zshrc
use_color: true
```
Example config for Linux/bash:

```yaml
shell_file: /home/yourname/.bashrc
storage_path: /home/yourname/.config/gloss
scan_paths:
  - /home/yourname/.bashrc
  - /home/yourname/.bash_aliases
use_color: true
```

### Config fields

- `shell_file` — shell file used for managed alias sync
- `storage_path` — location of the SQLite DB and config file
- `scan_paths` — extra files/directories to scan
- `use_color` — basic color preference

## Install paths

### Binary

Typical install location:

```bash
/usr/local/bin/gloss
```

### Config/data

Typical runtime location:

```bash
~/.config/gloss/
```

## 🎯 Supported workflow

Gloss is best suited for people who:

- keep useful shell aliases but forget them later
- want a simple personal command glossary
- want a lightweight terminal UI instead of a docs file
- want managed aliases in a dedicated safe block
- use zsh on macOS or bash on Linux

## What Gloss is not

Gloss is intentionally **not**:

- a shell replacement
- a shell history analyzer
- a package manager
- an AI command explainer
- a full shell plugin manager
- a cloud sync product

It is a small local utility for documenting and managing useful commands.

## 👨🏻‍💻 Development

Clone the repo:

```bash
git clone https://github.com/Architeg/gloss.git
cd gloss
```

Run locally:

```bash
go run ./cmd/gloss
```

Build:

```bash
go build ./cmd/gloss
```

Check version:

```bash
go run ./cmd/gloss version
```

## Release assets

GitHub Releases provide the official binaries for:

- `darwin-arm64`
- `darwin-amd64`
- `linux-amd64`
- `linux-arm64`

These release assets are used by:

- manual installs
- the install script
- the Homebrew formula

## Roadmap ideas

Possible future improvements:

- cleaner CLI output formatting
- editable settings in TUI
- richer alias management
- broader shell support beyond zsh/bash
- shell completions
- import/export helpers
- `gloss --version` metadata with commit/date
- more polished release automation

## ⭐ Support Gloss

If Gloss saves you time or becomes part of your workflow, you can [share it](https://twitter.com/intent/tweet?url=https://github.com/Architeg/gloss&text=Gloss%20%E2%80%94%20A%20small%20command%20glossary%20for%20your%20terminal.), maybe [give it a star](https://github.com/Architeg/gloss/stargazers), or support the project here:

[![GitHub Sponsors](https://img.shields.io/badge/GitHub%20Sponsors-support-ea4aaa?logo=githubsponsors&logoColor=white)](https://github.com/sponsors/Architeg)  
[![Ko-fi](https://img.shields.io/badge/Ko--fi-support-FF5E5B?logo=kofi&logoColor=white)](https://ko-fi.com/architeg)

## Contributing

Issues, suggestions, and small focused PRs are welcome.

If you contribute:

- keep the UI restrained
- prefer simple and readable code
- avoid unnecessary abstraction
- avoid feature creep for the core workflow

Thanks to everyone who contributes to Gloss. ❤️

<a href="https://github.com/Architeg/gloss/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Architeg/gloss" alt="Gloss contributors"/>
</a>

## License

MIT

See [LICENSE](./LICENSE).
