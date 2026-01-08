# stk - Stacked Branches CLI

A command-line tool for managing stacked branches (stacked diffs) in Git.

## What are Stacked Branches?

Stacked branches are a workflow where you create a chain of dependent branches, each building on top of the previous one. This is useful for:

- Breaking large features into smaller, reviewable PRs
- Keeping dependent changes in sync during development
- Parallel code review of related changes

```
main
 └── feature/auth-models     ← PR #1: Data models
      └── feature/auth-api   ← PR #2: API endpoints (depends on #1)
           └── feature/auth-ui ← PR #3: UI components (depends on #2)
```

## Installation

```bash
# From source
go install github.com/stefanaki/stk/cmd/stk@latest

# Or clone and build
git clone https://github.com/stefanaki/stk
cd stk
make install
```

## Quick Start

```bash
# Initialize a stack on your current branch
stk init my-feature --base main

# Create branches as you work
stk branch auth-models
# ... make changes, commit ...

stk branch auth-api
# ... make changes, commit ...

stk branch auth-ui
# ... make changes, commit ...

# See your stack
stk status

# Sync with remote (fetch, update base, rebase stack)
stk sync

# Push all branches and create/update PRs
stk submit
```

## Commands

### Stack Management

| Command | Description |
|---------|-------------|
| `stk init <name>` | Initialize a new stack |
| `stk status` | Show current stack status |
| `stk list` | List all stacks |
| `stk switch <name>` | Switch to a different stack |
| `stk delete <name>` | Delete a stack |
| `stk rename <old> <new>` | Rename a stack |
| `stk doctor` | Validate stack integrity |
| `stk log` | Show stack as a tree |

### Branch Operations

| Command | Description |
|---------|-------------|
| `stk branch <name>` | Create a new branch and add to stack |
| `stk add <branch>` | Add existing branch to stack |
| `stk remove <branch>` | Remove branch from stack |
| `stk move <branch> --after <other>` | Reorder branch in stack |

### Navigation

| Command | Description |
|---------|-------------|
| `stk up` | Checkout parent branch |
| `stk down` | Checkout child branch |
| `stk top` | Checkout base branch |
| `stk bottom` | Checkout last branch |
| `stk goto <n>` | Checkout nth branch |
| `stk which` | Show current position |

### Sync & Submit

| Command | Description |
|---------|-------------|
| `stk sync` | Fetch, refresh PR states, cleanup merged/closed, rebase |
| `stk sync --no-fetch` | Local rebase only (skip fetching) |
| `stk sync --no-rebase` | Only refresh PR states, don't rebase |
| `stk sync --delete-merged` | Delete local branches for merged PRs |
| `stk submit` | Push all branches, create/update PRs |
| `stk submit --draft` | Create new PRs as drafts |
| `stk submit --no-create-prs` | Push only, don't create new PRs |
| `stk submit --no-update-prs` | Don't update existing PR descriptions |
| `stk edit [branch]` | Interactive rebase within a branch |

### Pull Requests

| Command | Description |
|---------|-------------|
| `stk pr status` | Show PR status for all branches |
| `stk pr status --refresh` | Refresh PR status from remote |
| `stk pr view [branch]` | Open PR in browser |
| `stk pr create [branch]` | Manual PR creation (usually use `stk submit` instead) |
| `stk pr update [branch]` | Manual PR description update |

> **Note:** PR merging and closing should be done via GitHub/GitLab UI.
> When you run `stk sync`, it automatically detects merged/closed PRs and updates the stack accordingly.

### Workflow

The CLI follows a **sync → submit** workflow:

**`stk sync`** (remote → local):
1. Fetch updates from origin
2. Update base branch (pull --rebase)
3. Refresh PR states from remote
4. Process merged PRs (remove from stack, retarget downstream PRs)
5. Process closed PRs (clear metadata, will recreate on submit)
6. Rebase entire stack onto updated base

**`stk submit`** (local → remote):
1. Check if base branch is synced with remote
2. Push all branches (--force-with-lease)
3. Create PRs for branches without one
4. Update all PR descriptions with current stack status

Each PR description includes a "Stack" section showing:
- All branches in the stack
- PR numbers and links
- Current status of each PR
- Which PR you're currently viewing

Example PR description:

```markdown
## 📚 Stack

This PR is part of the **my-feature** stack:

| # | Branch | PR | Status |
|---|--------|-----|--------|
| 1 | `feature/auth-models` | #142 | ✅ Merged |
| **2** | **`feature/auth-api`** | **#143** | **🔄 This PR** |
| 3 | `feature/auth-ui` | #144 | 📝 Draft |

---
*Managed by [stk](https://github.com/stefanaki/stk)*
```

## Atomic Rebases

The rebase during `stk sync` is atomic - if any rebase fails (e.g., due to conflicts), all branches are automatically rolled back to their original state.

```
📸 Saving branch positions for rollback...

▶ Rebasing feature/auth-models onto main
▶ Rebasing feature/auth-api onto feature/auth-models

❌ Rebase failed.

⏪ Rolling back all branches...
  Resetting feature/auth-models to e5f6g7h8
  Resetting feature/auth-api to i9j0k1l2

✅ Rollback complete - stack restored to original state
```

After resolving conflicts manually, use `stk sync --no-fetch` to retry the rebase.

## Stack Storage

Stacks are stored in `.git/stacks/<name>.yaml`:

```yaml
version: 1
name: my-feature
base: main
created: 2026-01-04T10:30:00Z
updated: 2026-01-04T14:22:00Z
branches:
  - name: feature/auth-models
    pr:
      number: 142
      url: https://github.com/org/repo/pull/142
      state: open
  - name: feature/auth-api
    pr:
      number: 143
      url: https://github.com/org/repo/pull/143
      state: open
```

## Shell Completion

```bash
# Bash
stk completion bash > /etc/bash_completion.d/stk

# Zsh
stk completion zsh > "${fpath[1]}/_stk"

# Fish
stk completion fish > ~/.config/fish/completions/stk.fish
```

## Requirements

- Git 2.0+
- Go 1.21+ (for building from source)
- GitHub CLI (`gh`) for PR operations (optional, can use `GITHUB_TOKEN` instead)

## License

MIT
