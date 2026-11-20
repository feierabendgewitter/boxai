# boxai

[Deutsch](README.md) | **English**

`boxai` bundles several Git repos as worktrees into one workspace and launches it in a [Docker Sandbox](https://docs.docker.com/ai/sandboxes/) with Claude Code, GitHub Copilot CLI or (experimental) Google Antigravity. An agent can work across repos while staying isolated from the host.

## Requirements

- macOS or Linux, each with `git`
- `sbx` v0.51 or later: `brew install docker/tap/sbx`, then `sbx login`
- Docker only for `make images` and `--dockerfile`

## Installation

From a [release](https://github.com/feierabendgewitter/boxai/releases) or with Go:

```sh
go install github.com/feierabendgewitter/boxai/cmd/boxai@latest
boxai doctor
```

## First run

```sh
cd ~/projects          # folder containing your repos
boxai up               # prompts for name, repos and agent
```

Without custom images the agent starts with its default image; `boxai` points out which tools are missing there. `make images` builds the images with profiles from this repository ([details](.okf/runbooks/images-bauen.md)).

Non-interactive:

```sh
boxai up --name shop --repo backend --repo frontend --agent claude --profile kotlin-spring,react
boxai up --config boxai.yaml      # template: boxai init, example: examples/
boxai up --dry-run ...            # shows all git and sbx commands
```

## Commands

| Command | Purpose |
| --- | --- |
| `boxai up` | Create worktrees and sandbox, start the agent; running again resumes |
| `boxai scan` | List repos under a folder |
| `boxai list` | Workspaces and sandbox status |
| `boxai attach <name>` | Restart the agent |
| `boxai stop <name>` | Stop the sandbox |
| `boxai rm <name>` | Remove sandbox, worktrees, kit and state; branches are kept unless `--delete-branches` is given |
| `boxai doctor` | Check prerequisites |
| `boxai init` | Write `boxai.yaml` and `secrets.example.yaml` |
| `boxai agent-help` | Print instructions for AI agents that operate boxai |

## Good to know

Through the mount of `<repo>/.git`, the agent can modify objects and branches of your main repos ([why](.okf/decisions/0005-worktree-mounts.md)). Secret values stay in the sbx store; the sandbox only sees placeholders ([details and exceptions](.okf/domain/secrets.md)).

## Documentation

The project knowledge lives as an OKF bundle in [`.okf/`](.okf/index.md) (in German). Agents that develop boxai start at [`AGENTS.md`](AGENTS.md); agents that operate boxai start at `boxai agent-help`.

## License

[MIT](LICENSE)
