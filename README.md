# bashq

**bashq** is a minimalist TUI agent for Linux that translates natural language directly in your terminal into precise shell command chains — and executes them locally.

```
  bashq                                                        [ ASK ]
 ──────────────────────────────────────────────────────────────────────
  Assistent
  Ich schaue mal, welche Pakete aktualisiert werden können.

  ┌─────────────────────────────────────────────────────────────┐
  │  $ apt list --upgradable 2>/dev/null | head -20             │
  │  Zeigt aktualisierbare Pakete (erste 20)                    │
  │  J/Enter Ausführen   N/Esc Abbrechen                        │
  └─────────────────────────────────────────────────────────────┘
```

## Features

- **Natural language → shell commands** – describe what you want, bashq figures out the commands
- **Confirmation prompt** – always shows the command and a plain-language explanation before running
- **Auto mode** – skip prompts for repetitive workflows (`Shift+Tab` to toggle)
- **LLM profile auto-discovery** – enter an IP, bashq scans common ports and detects models automatically
- **Multi-language UI** – German, English, Simplified Chinese (auto-detected from system locale)
- **Zero install** – single statically linked binary, no dependencies
- **Activity log** – full history of every command and response
- **F1–F9 shortcuts** – one-key macros for your most-used queries
- **Custom system prompt** – override the assistant's personality and focus

## Quick Start

### Download

Grab the latest binary from the [Releases](../../releases) page.

```bash
chmod +x bashq
./bashq
```

### First run

On startup bashq connects to `http://localhost:9081/v1` (default). Open `/config` to change the endpoint or run auto-discovery.

## Requirements

Any **OpenAI-compatible** local LLM server:

| Server | Default port |
|--------|-------------|
| [Ollama](https://ollama.com) | 11434 |
| [LM Studio](https://lmstudio.ai) | 1234 |
| [vLLM](https://github.com/vllm-project/vllm) | 8000 |
| [llama.cpp server](https://github.com/ggerganov/llama.cpp) | 8080 |
| [Jan](https://jan.ai) | 1234 |

## Configuration

Type `/config` in the input field to open the settings editor.

### LLM Profile Auto-Discovery

1. Open `/config` → **LLM PROFILES** section
2. Select `[ + New LLM Profile ]` and press `Enter`
3. Enter the IP address or hostname of your LLM server
4. bashq scans ports `11434 1234 8080 8000 9081 7860 5000 3000` automatically
5. Select a model from the detected list
6. Give the profile a name — done

Multiple profiles can be saved. Mark one as preferred (`P`) and bashq will warn you on startup if it is unreachable and suggest the next available profile.

### Config file

`~/.config/bashq/config.json` — written automatically, human-readable JSON.

### Activity log

`~/.config/bashq/activities.log` — every query, response and executed command.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message / confirm command |
| `↑ / ↓` | Scroll chat / navigate lists |
| `Shift+Tab` | Toggle Ask ↔ Auto execution mode |
| `F1 – F9` | Custom shortcuts (configure in `/config`) |
| `Esc` | Close autocomplete / cancel |
| `Ctrl+C` | Quit |

## Slash Commands

Type `/` for autocomplete.

| Command | Description |
|---------|-------------|
| `/install` | Install software |
| `/update` | Update system |
| `/status` | System overview |
| `/disk` | Disk usage |
| `/memory` | Memory usage |
| `/network` | Network info |
| `/logs` | System logs |
| `/config` | Open settings |
| `/activities` | Show activity log |
| `/clear` | Clear chat history |

## Building from Source

Requires Go 1.22+.

```bash
git clone https://github.com/dev-core-busy/bashq.git
cd bashq
bash build.sh          # static binary → ./bashq
# or for a quick dev build:
go build -o bashq .
```

`build.sh` sets `CGO_ENABLED=0` for a fully portable, statically linked binary.

## License

MIT — see [LICENSE](LICENSE).
