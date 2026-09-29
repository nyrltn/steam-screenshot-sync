# steam-screenshot-sync

Steam stores screenshots under
`~/.local/share/Steam/userdata/<account>/760/remote/<appid>/screenshots/`, named only by
app id, and they are gone once that folder is cleared. This tool hard-links every screenshot
into `~/Pictures/Steam/<Game Name>/`.

A hard link is a second name for the same file, so it takes no extra space and keeps the
picture alive when Steam's copy is deleted. When the destination is on another filesystem,
the tool copies instead.

It runs once and exits. A systemd user timer runs it two minutes after login and every ten
minutes after that.

## Install

From the rpm:

```sh
sudo dnf install golang go-rpm-macros rpm-build systemd-rpm-macros
make rpm
sudo dnf install dist/rpmbuild/RPMS/x86_64/steam-screenshot-sync-*.rpm
```

The package enables the timer for every user through a systemd user preset. Users who were
already logged in when the package was installed can start it right away:

```sh
systemctl --user daemon-reload
systemctl --user enable --now steam-screenshot-sync.timer
```

Without the rpm, `make install-user` puts the binary in `~/.local/bin` and enables the timer.

## Usage

```sh
steam-screenshot-sync --dry-run -v              # show what would happen
systemctl --user start steam-screenshot-sync    # sync now
journalctl --user -u steam-screenshot-sync      # see results
```

| Flag | Default |
|---|---|
| `--steam-dir` | `$STEAM_DIR`, then `~/.local/share/Steam`, `~/.steam/steam`, `~/.steam/root`, the Flatpak path |
| `--dest` | `XDG_PICTURES_DIR` from `~/.config/user-dirs.dirs` plus `/Steam`, else `~/Pictures/Steam` |
| `--cache` | `~/.cache/steam-screenshot-sync/names.json` |
| `--dry-run`, `-v`, `--version` | |

## Behaviour

- Game names come from `appmanifest_*.acf` in every Steam library. They are cached, so an
  uninstalled game keeps its folder name. An app with no known name gets a folder named after
  its app id.
- A game folder is created only when the game has at least one screenshot. Thumbnails are
  skipped.
- Nothing is ever deleted or overwritten. A different file already sitting under the same
  name is reported as a conflict and left alone.
- If a game is renamed in Steam, new screenshots go to a new folder and the old one stays.
- Screenshots from several Steam accounts on one machine are merged into the same game folder.
