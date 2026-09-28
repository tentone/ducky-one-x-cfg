# Ducky One X Configurator

A native, offline desktop configurator for the Ducky One X keyboard. It is written in Go with Fyne and communicates directly with the keyboard's vendor HID interface—no browser, account, or network connection is needed at runtime.

## Features

- Windows, Linux, and macOS desktop UI
- Automatic USB discovery, connection, firmware identification, and configuration loading
- Switch between both onboard memory profiles and immediately load the selected profile
- Visual full-size keyboard with click-to-edit Base and Fn key assignments
- Animated on-keyboard lighting previews for static, breathing, cycle, reactive, ripple, rainbow, analog, and off effects
- Per-key actuation overlays, click feedback, and rapid-trigger settings across all 126 matrix positions
- Four-stage multi-point-trigger presets (MPT1–MPT14)
- Fourteen onboard macro slots with press, release, click, delay, and text actions
- System, light, and dark themes
- Translation catalog architecture, with English included

The protocol was translated from the saved Ducky One X web configurator in `webapp_snapshot`. Runtime code does not load anything from that snapshot or contact `duckyhub.io`.

## Run from source

Install Go 1.24 or later and a C compiler, then run:

```sh
go run ./cmd/ducky-config
```

Fyne and the HID library both use CGo. Platform development prerequisites are:

- **Windows:** MSYS2/MinGW-w64 (recommended by Fyne), or Zig. With Zig in PowerShell:

  ```powershell
  $env:CGO_ENABLED = "1"
  $env:CC = "zig cc"
  go run ./cmd/ducky-config
  ```

- **macOS:** install the Xcode command-line tools with `xcode-select --install`.
- **Debian/Ubuntu:**

  ```sh
  sudo apt-get install golang gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev libudev-dev
  ```

On Linux, install the included udev rule so the logged-in desktop user can open the HID interface:

```sh
sudo install -m 0644 packaging/99-ducky-one-x.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
sudo udevadm trigger
```

Reconnect the keyboard after installing the rule.

## Build

Build for the current operating system:

```sh
./scripts/build.sh
```

On Windows PowerShell:

```powershell
./scripts/build.ps1
```

The output is placed in `dist/`. Native builds are recommended because Fyne and HIDAPI require target-platform C toolchains when cross-compiling.

## Tests

The protocol and serialization tests do not need a keyboard:

```sh
go test ./internal/protocol ./internal/i18n
```

Run the full test/build check on a machine with the native GUI prerequisites:

```sh
go test ./...
```

## Safety and hardware scope

The application only enumerates HID interfaces with Ducky's vendor ID `0x3233` and configuration usage page `0x008c`. Writes are serialized and responses are matched to the expected command. Settings still change persistent keyboard memory, so test new macro and analog settings before relying on them in critical workflows.

This is an independent project and is not affiliated with DuckyChannel International Co., Ltd.

See [docs/protocol.md](docs/protocol.md) for the implemented command map and [docs/architecture.md](docs/architecture.md) for the code layout.
