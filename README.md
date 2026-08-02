# svrctl

A CLI for running local Minecraft servers — create, start, stop, and watch
them without hand-managing jars, Java versions, or `screen` sessions.

```
$ svrctl
 svrctl  2 servers

  NAME       TYPE      VERSION   STATUS      UPTIME
› survival   paper     1.21.4    ● running   1h 30m
  creative   vanilla   1.21.1    ○ stopped   —

  ↑↓ select  s start  x stop  r restart  c console  l logs  d remove  n new  q quit
```

Run `svrctl` with no arguments to open that dashboard. Everything below also
works as a plain command, for scripts and muscle memory.

## Install

Requires Go 1.26+.

```
go install github.com/adammcgrogan/svrctl/cmd/svrctl@latest
```

Or build from a checkout:

```
git clone https://github.com/adammcgrogan/svrctl
cd svrctl
go build -o svrctl ./cmd/svrctl
```

## Quick start

```
svrctl create survival --type paper --version 1.21.4 --accept-eula
svrctl start survival
svrctl console survival
```

Or just run `svrctl create` on a terminal and answer the questions — it walks
you through picking a type, a real published version, memory, and a port.

## Commands

| Command | Does |
|---|---|
| `svrctl create [name]` | Set up a new server — downloads the jar, installs the Java version it needs, registers it |
| `svrctl list` | List your servers |
| `svrctl status <name>` | Show everything about one server |
| `svrctl start <name>` | Start a server in the background (`--attach` to stay in the foreground) |
| `svrctl stop <name>` | Stop a running server (`--force` to kill it, `--timeout` to adjust the grace period) |
| `svrctl restart <name>` | Stop and start it |
| `svrctl console <name>` | Attach to a running server's console, scrollback and all |
| `svrctl cmd <name> <command...>` | Send one command without attaching |
| `svrctl logs <name>` | Show recent output (`-f` to follow, `-n` for how much) |
| `svrctl versions` | List Minecraft versions you can pass to `create --version` |
| `svrctl remove <name>` | Unregister a server (`--purge` to also delete its files) |

Every command takes `--json` where a machine might read the output, and
`--plain` disables colour and interactive screens globally.

## Where things live

svrctl keeps three things on disk, all in OS-standard locations:

- **`servers.yaml`** — the registry of what you've created, under your user
  config directory (e.g. `~/Library/Application Support/svrctl` on macOS,
  `~/.config/svrctl` on Linux, `%AppData%\svrctl` on Windows).
- **Cached JDKs and server jars** — under your user cache directory, so a
  second server on the same Java version doesn't download it twice.
- **Server files** — `~/mcservers/<name>` by default, or wherever `--path`
  points.

## Scope

Runs on macOS, Linux, and Windows. Vanilla and Paper are supported today;
no plugin or mod management yet.
