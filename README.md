# gommit

Generate git commit messages using an OpenAI-compatible LLM.

## Install

Linux (amd64) from GitHub Releases:

1. Download the `gommit_*_linux_amd64.tar.gz` or `gommit_*_linux_amd64.deb` asset from the release page.
2. For tar.gz:

```bash
tar -xzf gommit_*_linux_amd64.tar.gz
sudo install -m 0755 gommit /usr/local/bin/gommit
```

3. For .deb:

```bash
sudo dpkg -i gommit_*_linux_amd64.deb
```

Build from source:

```bash
go build -o gommit
```

Install with Go:

```bash
go install github.com/MenschMachine/gommit@latest
```

## Usage

```bash
# staged only (default)
./gommit --provider openai --model gpt-4o-mini

# staged + unstaged
./gommit -a --provider openai --model gpt-4o-mini

# staged + unstaged + untracked
./gommit -A --provider openai --model gpt-4o-mini
```

## Flags

- `-a`: include staged + unstaged
- `-A`: include staged + unstaged + untracked
- `--single`: force single-message mode even if diff is large
- `--split`: force split-mode (multi-commit plan)
- `-f`, `--accept`: auto-accept proposed result (skips prompt)
- `--provider`: `openai`, `openrouter`, `anthropic`
- `--model`: model name (required unless set in config)
- `--base-url`: OpenAI-compatible base URL
- `--style`: `conventional` or `freeform`
- `--config`: config file path
- `--openrouter-referer`: set OpenRouter `HTTP-Referer` header
- `--openrouter-title`: set OpenRouter `X-Title` header

## Config

Default config path: `~/.config/gommit/config.toml`

Example:

```toml
provider = "openai"
model = "gpt-4o-mini"
base_url = "https://api.openai.com/v1"
style = "conventional"
split_threshold = 200000
per_file_limit = 20000
openrouter_referer = "https://example.com"
openrouter_title = "gommit"
```

## Environment Variables

API keys:

- `OPENAI_API_KEY`
- `OPENROUTER_API_KEY`
- `ANTHROPIC_API_KEY`
- `GOMMIT_API_KEY` (fallback)

Config overrides:

- `GOMMIT_PROVIDER`
- `GOMMIT_MODEL`
- `GOMMIT_BASE_URL`
- `GOMMIT_STYLE`
- `GOMMIT_SPLIT_THRESHOLD`
- `GOMMIT_PER_FILE_LIMIT`
- `GOMMIT_OPENROUTER_REFERER`
- `GOMMIT_OPENROUTER_TITLE`
- `OPENROUTER_REFERER`
- `OPENROUTER_TITLE`
