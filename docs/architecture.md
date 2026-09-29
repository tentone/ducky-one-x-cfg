# Architecture

The application is split into four layers:

- `internal/protocol` contains packet construction, response decoding, feature models, macro serialization, and analog position conversion. It has no GUI or native HID dependency and is unit tested.
- `internal/device` owns HIDAPI initialization, filters the Ducky configuration interface, pads reports to the descriptor size, serializes exchanges, and ignores unrelated asynchronous reports while waiting for a command response.
- `internal/profiles` validates complete keyboard snapshots and atomically persists an unlimited named profile library as JSON in the user's configuration directory.
- `internal/ui` contains the Fyne window, theme and language preferences, connection state, and configuration screens. Device work runs outside Fyne's UI goroutine and returns UI updates through `fyne.Do`.
- `internal/i18n` keeps all interface strings behind stable identifiers. Adding a language means adding another catalog map and exposing it in the language selector; feature code does not need to change.

The application has no runtime network client and no telemetry.
