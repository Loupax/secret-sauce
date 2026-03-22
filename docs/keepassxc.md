# KeePassXC setup (Sway / minimal Wayland)

KeePassXC can act as a Secret Service provider over D-Bus, which is how `secret-sauce`
stores your private key without writing it to disk. It requires a few steps beyond
installing and enabling the toggle.

---

## 1. Enable Secret Service integration

*Tools → Settings → Secret Service Integration → Enable KeePassXC Secret Service
Integration*

Restart KeePassXC after toggling this — the setting does not take effect until restart.

---

## 2. Expose at least one group

On the same settings page, check at least one database group under
*"Expose entries of group"*. Without an exposed group KeePassXC does not register a
default collection on D-Bus, and `secret-sauce` will fail with:

```
failed to unlock correct collection '/org/freedesktop/secrets/aliases/default'
```

---

## 3. Keep a database open and unlocked

The Secret Service is only available while KeePassXC has an unlocked database. If you
lock the database or close KeePassXC, any subsequent `secret-sauce` command will fail
until you unlock it again via the tray icon.

---

## 4. Grant access when prompted

The first time `secret-sauce` contacts the Secret Service, KeePassXC will show an
access-request dialog. Bring the KeePassXC window to the front and click *Allow*.

---

## 5. Verify the setup is working

```bash
busctl --user call org.freedesktop.secrets \
  /org/freedesktop/secrets/aliases/default \
  org.freedesktop.DBus.Peer Ping
```

A successful reply means the default collection is reachable and `secret-sauce init`
should work.

---

## Running KeePassXC as a systemd user service

By default KeePassXC stops when you close its window, taking the Secret Service down
with it. Running it as a systemd user service keeps it alive in the background for the
lifetime of your session.

### Create the service file

```bash
mkdir -p ~/.config/systemd/user
```

Create `~/.config/systemd/user/keepassxc.service`:

```ini
[Unit]
Description=KeePassXC password manager (Secret Service)
After=graphical-session.target
PartOf=graphical-session.target

[Service]
ExecStart=/usr/bin/keepassxc --minimized
Restart=always
RestartSec=3
Environment=QT_QPA_PLATFORM=wayland

[Install]
WantedBy=graphical-session.target
```

### Enable and start it

```bash
systemctl --user daemon-reload
systemctl --user enable --now keepassxc.service
```

### Check it is running

```bash
systemctl --user status keepassxc.service
```

### Grant access before running headless (required, one-time)

When running as a service with no visible window, KeePassXC cannot display the
access-request dialog that appears the first time an app connects to the Secret
Service. You must grant access once through the GUI before switching to headless mode.

```bash
# Stop the service so there is no conflicting instance
systemctl --user stop keepassxc.service

# Launch KeePassXC normally so the window appears
keepassxc &

# Trigger the access prompt from another terminal
secret-sauce ls

# Click Allow in the KeePassXC dialog that appears
# KeePassXC writes the approval into the database — it will not ask again

# Quit KeePassXC from its window, then restart the service
systemctl --user start keepassxc.service
```

After this, the headless service has permanent access and `secret-sauce` works without
any dialogs.

### Sway: no system tray

Sway does not support the XDG system tray protocol natively, so KeePassXC's tray icon
will not appear. The `--minimized` flag is still useful to suppress the startup window,
but there is no tray to interact with. To stop KeePassXC use
`systemctl --user stop keepassxc.service`; to open the window temporarily (e.g. to
unlock the database) run `keepassxc` directly while the service is stopped.

### Notes

- `Restart=always` restarts KeePassXC regardless of exit code — including if you
  accidentally close it from the tray. To stop it permanently, use
  `systemctl --user stop keepassxc.service`.
- The database will still lock on timeout according to your KeePassXC security
  settings. When it locks, `secret-sauce` will fail until you unlock it again via the
  tray icon. Adjust the lock timeout under *Tools → Settings → Security* to suit your
  threat model.
- `QT_QPA_PLATFORM=wayland` tells Qt to use the native Wayland backend. Remove this
  line if you are running an X11 session or if KeePassXC fails to start.
- You may see repeated journal warnings: `qt.qpa.wayland: Wayland does not support
  QWindow::requestActivate()`. These are harmless — Wayland intentionally prevents apps
  from stealing focus and Qt just logs it. Suppress them by adding
  `Environment=QT_LOGGING_RULES=qt.qpa.wayland.warning=false` to the `[Service]`
  section.
