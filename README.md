# Geek-life

Geek-life is a fast, local, keyboard-first task manager that stays inside your
terminal. It combines projects, Markdown notes, structured search, recurring
tasks, task metadata, recoverable deletion, backups, and clipboard images in a
single Go binary.

## What makes it powerful

- Fuzzy command palette with `:` or `Ctrl+P`
- Cross-project search with `/`
- Vim-style list navigation and complete mouse-free workflows
- Quick capture such as `Ship release #work !3 @tomorrow ~weekly +pin`
- Tags, priorities, pins, due dates, recurrences, and smart lists
- Trash, keyboard undo, restore, and confirmed permanent deletion
- Markdown notes with in-app and external-editor workflows
- Windows clipboard-image paste with automatic Sixel, Kitty, and iTerm native-pixel backends
- Portable cell-based color/detail viewer with zoom/pan as a fallback
- JSON snapshots from the UI or CLI

Press `?` inside the app for the complete keyboard reference.

## Windows installation

From PowerShell in the project directory:

```powershell
.\install.ps1
```

The installer finds Go in `PATH` and common Windows locations such as
`C:\Program Files\Go\bin\go.exe`. If Go is unavailable, it can install an
existing prebuilt `geek-life.exe` from the project directory.

Useful options:

```powershell
.\install.ps1 -GoPath "C:\Program Files\Go\bin\go.exe"
.\install.ps1 -BinDir "$HOME\tools" -SkipPathUpdate
```

The default destination is `%USERPROFILE%\bin\geek-life.exe`. Open a new
terminal after installation, then run `geek-life`.

## Linux/macOS installation

```bash
chmod +x ./install.sh
./install.sh
```

The default destination is `$HOME/.local/bin/geek-life`. The script can also
install a prebuilt `geek-life` binary when Go is unavailable.

## Essential keys

| Key | Action |
| --- | --- |
| `:` / `Ctrl+P` | Command palette |
| `/` | Structured search |
| `?` | Full keyboard help |
| `p` / `t` | Focus projects / tasks |
| `j` / `k` | Move down / up |
| `Enter` / `Esc` | Open / back |
| `Ctrl+N` | Quick new task |
| `r` in project actions | Rename the current project |
| `Space` | Complete or resume task |
| `!` / `s` | Cycle priority / pin |
| `D` / `u` | Trash / undo or restore |
| `i` / `Ctrl+V` | Paste clipboard image into task |
| `o` / `O` | Real-pixel image / compatibility cell viewer |

Lowercase `o` selects the best real-pixel protocol advertised by the terminal:
Sixel in Windows Terminal, Kitty graphics in Kitty/WezTerm/Ghostty, or iTerm
images in iTerm2/mintty/Rio. Press Enter to return. Uppercase `O` deliberately
uses the portable cell renderer; use `b` to toggle the font-dependent sextant
mode, `1`-`9` to zoom, arrows or `h/j/k/l` to pan, and `f` to fit.

Run `geek-life doctor` to see the selected backend. Detection can be overridden
with `GEEK_LIFE_IMAGE_PROTOCOL=sixel`, `kitty`, `iterm`, or `cell`. An override
cannot add a protocol the terminal emulator itself does not implement.

## Search examples

Search terms and filters can be combined:

```text
installer #work @geek !2 is:open due:overdue
```

Filters include `#tag`, `@project`, minimum priority (`!2`), `is:done`,
`is:open`, `is:pinned`, `is:recurring`, and date filters such as
`due:today`, `due:overdue`, `due:none`, or `due:2026-08-14`.

## CLI utilities

```text
geek-life backup [file]   Write a JSON snapshot
geek-life migrate         Rebuild indexes and backfill legacy metadata
geek-life doctor          Show terminal/image backend diagnostics
geek-life version         Print the installed version
geek-life help            Show CLI help
```

By default, data lives at `~/.geek-life/default.db`. Set `DB_FILE` to choose a
different database. Pasted images and automatic snapshots live beside it.
JSON snapshots store image paths rather than image bytes, so include the
neighboring `attachments` directory in a full disaster-recovery backup.

For the extended guide and project history, see
[Geek-Life-README.md](Geek-Life-README.md).
