# secret-sauce GUI

A graphical vault browser for [secret-sauce](../README.md), built with [Wails](https://wails.io/) and Svelte.

## Behaviour

- **Window starts hidden.** The app runs silently in the system tray on launch.
- **Left-click the tray icon** → shows and focuses the vault window (macOS / Windows).
  On Linux (StatusNotifier / appindicator), any click opens the context menu instead.
- **Context menu** contains:
  - *Show Vault* — shows and focuses the vault window (Linux fallback)
  - *Quit* — exits the application
- **Closing the window** hides it rather than quitting; the app stays resident in the tray.

## Development

```bash
cd gui
wails dev
```

This starts a Vite dev server with hot-reload for frontend changes.
The Go backend is also recompiled on change.
A browser dev server is available at `http://localhost:34115`.

## Building

```bash
cd gui
wails build
```

Produces a single self-contained binary at `gui/build/bin/sauce-gui`.

### Linux build dependencies

```bash
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev libayatana-appindicator3-dev
```

On Arch:
```bash
sudo pacman -S webkit2gtk-4.1 libayatana-appindicator
```

## System tray notes

The tray icon uses the [StatusNotifierItem](https://www.freedesktop.org/wiki/Specifications/StatusNotifierItem/)
D-Bus protocol (`fyne.io/systray`). Your desktop environment must have a StatusNotifier-aware
system tray:

| DE | Support |
|---|---|
| KDE Plasma | Built-in |
| GNOME | Requires the [AppIndicator](https://extensions.gnome.org/extension/615/appindicator-support/) extension |
| Sway / wlroots | Requires [waybar](https://github.com/Alexays/Waybar) or similar with tray support |
