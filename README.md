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
go install github.com/gstefan/stk/cmd/stk@latest

# Or clone and build
git clone https://github.com/gstefan/stk
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

# After main is updated, rebase the entire stack
stk rebase

# Push all branches and create PRs
stk sync
stk pr create
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

### Rebase & Sync

| Command | Description |
|---------|-------------|
| `stk rebase` | Rebase entire stack (atomic) |
| `stk rebase --from <branch>` | Start from specific branch |
| `stk rebase --to <branch>` | Stop at specific branch |
| `stk sync` | Fetch, rebase, push, and update PRs |
| `stk sync --no-update-prs` | Sync without updating PR descriptions |
| `stk push` | Push all branches |
| `stk edit` | Interactive rebase within branch |

### Pull Requests

| Command | Description |
|---------|-------------|
| `stk pr create` | Create PRs for all branches |
| `stk pr create --draft` | Create as drafts |
| `stk pr status` | Show PR status for all branches |
| `stk pr status --refresh` | Refresh PR status from remote |
| `stk pr update` | Update all PR descriptions with stack info |
| `stk pr view [branch]` | Open PR in browser |
| `stk pr merge` | Merge first mergeable PR |
| `stk pr merge <branch>` | Merge specific PR |
| `stk pr merge --squash` | Use squash merge |
| `stk pr merge --delete` | Delete branch after merge |
| `stk pr close <branch>` | Close PR without merging |

### PR Workflow

When you run `stk sync`, it will:
1. Fetch updates from origin
2. Rebase base branch onto upstream
3. Rebase entire stack (atomic)
4. Push all branches
5. **Update all PR descriptions** with current stack status

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
*Managed by [stk](https://github.com/gstefan/stk)*
```

## Atomic Rebases

By default, `stk rebase` is atomic - if any rebase fails (e.g., due to conflicts), all branches are automatically rolled back to their original state.

```
📸 Saving branch positions for rollback...

▶ Rebasing feature/auth-models onto main
▶ Rebasing feature/auth-api onto feature/auth-models

❌ Rebase failed.

⏪ Rolling back all branches...
  Resetting main to a1b2c3d4
  Resetting feature/auth-models to e5f6g7h8
  Resetting feature/auth-api to i9j0k1l2

✅ Rollback complete - stack restored to original state
```

Use `--no-atomic` to disable this behavior.

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

## Configuration

Create `~/.stk.yaml` for global configuration:

```yaml
# Default settings
sync:
  no-push: false
```

Or use environment variables:

```bash
export STK_SYNC_NO_PUSH=true
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
