# Ducky One X HID protocol notes

The keyboard configurator interface is selected with vendor ID `0x3233` and usage page `0x008c`. Commands use report ID `0` and a payload framed as:

```text
66 <length> <command> <command data...> 0d 0a
```

`length` is command-specific; on chunked commands it describes the chunk payload rather than the whole frame. HIDAPI pads short output reports to the size declared by the device where the operating system requires it. Input reports may expose `0x66` as a report ID or as the first data byte, so the native transport normalizes both layouts.

## Implemented commands

| Request | Response | Purpose |
|---:|---:|---|
| `0x01` | `0x02` | Device and firmware metadata |
| `0x05` | `0x06` | Read lighting parameters |
| `0x07` | `0x08` | Write lighting parameters |
| `0x13` | `0x14` | Read active key layer |
| `0x15` | `0x16` | Read key-map chunk |
| `0x17` | `0x18` | Select active key layer |
| `0x19` | `0x1a` | Reset a key layer |
| `0x1b` | `0x1c` | Write key-map chunk |
| `0x1d` | `0x1e` | Read macro chunk |
| `0x1f` | `0x20` | Write macro chunk |
| `0x29` | `0x2a` | Read actuation chunk |
| `0x2b` | `0x2c` | Write actuation chunk |
| `0x2d` | `0x2e` | Set/reset actuation for all keys |
| `0x2f` | `0x30` | Read an MPT preset |
| `0x31` | `0x32` | Write an MPT preset |
| `0x33` | `0x34` | Reset all MPT presets |
| `0x9a` | `0x9b` | Read active memory profile |
| `0x9c` | `0x9d` | Switch memory profile |

Key maps have 126 two-byte entries and are transferred in five chunks. Actuation settings have 126 three-byte entries and are transferred in seven 54-byte chunks. Macros use a fixed 401-byte payload transferred in eight chunks. MPT presets contain four five-byte trigger stages.

Analog distances use the firmware's exact lookup: `0.1 mm → 13`, `0.2 mm → 14`, and `0.3–3.5 mm → distance × 70`.
