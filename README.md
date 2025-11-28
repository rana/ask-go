# ask

AI conversations through Markdown using AWS Bedrock Claude.

Ask treats markdown files as a source of truth. `ask` is a CLI tool that orchestrates conversations, but the markdown session is the primary artifact. Your thoughts, enriched with AI comprehension, become preserved knowledge.

---

## Quick Start

### Install

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/rana/ask/releases/latest/download/ask-v1.0.0-darwin-arm64.tar.xz | tar xJ
sudo mv ask /usr/local/bin/
```

**Linux (x86_64):**
```bash
curl -L https://github.com/rana/ask/releases/latest/download/ask-v1.0.0-linux-amd64.tar.xz | tar xJ
sudo mv ask /usr/local/bin/
```

**Windows (x86_64):**
1. Download `ask-v1.0.0-windows-amd64.zip` from [releases](https://github.com/rana/ask/releases)
2. Extract to desired location (e.g., `C:\Tools\ask`)
3. Add to PATH:
   ```powershell
   [Environment]::SetEnvironmentVariable(
     "Path", 
     "$env:Path;C:\Tools\ask", 
     [System.EnvironmentVariableTarget]::User
   )
   ```

**Windows (ARM64):**
Same as above, but download `ask-v1.0.0-windows-arm64.zip`

**From Source:**
```bash
go install github.com/rana/ask@latest
```

**Verify:**
```bash
ask --version
```

### Setup AWS

Configure AWS credentials with Bedrock access:

```bash
aws configure
```

**Or use a bearer token:**
```bash
export AWS_BEARER_TOKEN_BEDROCK="your-token-here"
```

### Start Thinking

```bash
ask init              # Creates session.md in current directory
```

Edit `session.md` with your preferred editor:

```markdown

I'm exploring distributed systems. 

[[architecture.md]]
[[patterns.md]]

What patterns emerge from these designs?
```

Run the conversation:

```bash
ask                   # Expands references, sends to Claude, appends response
```

Ask will:
1. Expand your `[[file]]` references into numbered sections
2. Send to Claude via AWS Bedrock  
3. Append the AI response as a new turn

Continue the conversation by adding a new human turn:

```markdown

Let's dive deeper into consensus mechanisms.

[[consensus.md]]
```

Run `ask` again to continue.

---

## Configuration

Ask stores configuration in `~/.ask/cfg.toml` (created automatically on first run).

### View Current Settings

```bash
ask cfg show
```

### Model Selection

```bash
ask cfg models           # List available Claude models
ask cfg model opus       # Use Claude Opus 4.5 (latest)
ask cfg model sonnet     # Use Claude Sonnet 3.5
ask cfg model haiku      # Use Claude Haiku 3.5
```

### Thinking Mode (Extended Reasoning)

Enable Claude's extended thinking capability:

```bash
ask cfg thinking on              # Enable thinking mode
ask cfg thinking off             # Disable
ask cfg thinking-budget 80%      # Allocate 80% of tokens to internal reasoning
```

When enabled, Claude uses extra tokens for deeper reasoning before responding.

### Context Windows

```bash
ask cfg context standard  # 200k tokens (default)
ask cfg context 1m        # 1 million tokens (Sonnet 4 only, requires AWS tier 4)
```

### Temperature and Tokens

```bash
ask cfg temperature 1.0   # Creativity level (0.0-1.0)
ask cfg max-tokens 32000  # Maximum response length
ask cfg timeout 5m        # Request timeout duration
```

---

## File References

### Basic References

```markdown
[[file.md]]              # Expands single file content
[[src/main.go]]          # Works with paths
```

### Directory Expansion

```markdown
[[src/]]                 # Expands directory (respects recursive config)
[[src/**/]]              # Force recursive expansion
[[internal/]]            # Non-recursive by default
[[internal/**/]]         # Force recursive
```

**Configure expansion behavior:**
```bash
ask cfg expand recursive on       # Make [[dir/]] recursive by default
ask cfg expand recursive off      # Require [[dir/**/]] for recursion
ask cfg expand max-depth 3        # Limit recursion depth (1-10)
```

### Included File Types

By default, `ask` includes:

- **Code:** `.go`, `.rs`, `.py`, `.js`, `.ts`, `.jsx`, `.tsx`, `.java`, `.cpp`, `.c`, `.h`, `.cs`, `.rb`, `.php`, `.swift`, `.kt`, `.scala`
- **Config:** `.json`, `.yaml`, `.yml`, `.toml`, `.xml`, `Makefile`, `Dockerfile`
- **Docs:** `.md`, `.txt`
- **Scripts:** `.sh`, `.bash`, `.zsh`, `.fish`, `.ps1`

### Excluded by Default

- **Directories:** `node_modules`, `.git`, `vendor`, `dist`, `build`, `target`, `bin`, `obj`, `.idea`, `.vscode`, `__pycache__`
- **Patterns:** `*_test.go`, `*.pb.go`, `*_generated.go`, `*.min.js`, `*.min.css`, `*.map`

Customize in `~/.ask/cfg.toml` under `[expand.include]` and `[expand.exclude]`.

---

## Content Filtering

Reduce token usage by stripping boilerplate:

```bash
ask cfg filter enable on            # Enable content filtering
ask cfg filter headers on           # Strip file headers (copyright, licenses)
ask cfg filter strip-comments on    # Remove all comments
```

**Preserved patterns** (even with stripping enabled):
- Directives: `//go:generate`, `// +build`, `#!`
- Lint annotations: `//nolint`, `//lint:`
- Encoding markers: `# -*- coding`, `# frozen_string_literal`

Configure patterns in `~/.ask/cfg.toml` under `[filter.header]`.

---

## Workflow Examples

### Exploring a Codebase

```markdown

Understanding the architecture of this project:

[[cmd/]]
[[internal/bedrock/]]
[[internal/session/]]

How do these components interact?
```

### Iterative Refinement

```markdown

The session parser needs to handle edge cases better.

[[internal/session/parser.go]]

What's missing in the regex handling?
```

### Documentation Generation

```markdown

Generate comprehensive API documentation for:

[[pkg/api/**/]]

Focus on public interfaces and include usage examples.
```

### Cross-File Analysis

```markdown

Compare these two implementations:

[[v1/handler.go]]
[[v2/handler.go]]

What improvements were made and why?
```

---

## AWS Setup

### Prerequisites

- AWS account with Bedrock access enabled
- Claude models activated in your AWS region
- Appropriate IAM permissions

### Configure Credentials

**Option 1: AWS CLI (Recommended)**
```bash
aws configure
```

Provide:
- Access Key ID
- Secret Access Key  
- Default region (e.g., `us-east-1`, `us-west-2`)

**Option 2: Bearer Token**

For temporary or token-based authentication:

```bash
# In ~/.bashrc or ~/.zshrc
export AWS_BEARER_TOKEN_BEDROCK="your-bearer-token-here"
```

**Option 3: Environment Variables**

```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_DEFAULT_REGION="us-east-1"
```

### Verify Access

```bash
ask cfg models  # Should list available Claude models
```

### Common Issues

**"No system inference profile found":**
- Enable cross-region inference in AWS Bedrock console
- Verify Claude models are activated in your region
- Try a different model: `ask cfg model sonnet`
- Check IAM permissions for Bedrock access

**"1M context requires tier 4 access":**
- Only Sonnet 4 supports 1M context windows
- Requires AWS tier 4 (higher usage tier)
- Solution: `ask cfg context standard` or upgrade AWS tier

**"AWS credentials not configured":**
- Run `aws configure` to set up credentials
- Or set `AWS_BEARER_TOKEN_BEDROCK` environment variable
- Verify credentials with: `aws sts get-caller-identity`

**"Profile may be stale, refreshing":**
- AWS inference profiles cached for 30 days
- Automatic refresh triggered on errors
- Manual cache clear: `rm -rf ~/.ask/cache/`

---

## Windows-Specific Notes

### PowerShell Execution Policy

If you see "cannot be loaded because running scripts is disabled":

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### PATH Configuration

**Temporary (current session only):**
```powershell
$env:Path += ";C:\Tools\ask"
```

**Permanent (user-level):**
```powershell
[Environment]::SetEnvironmentVariable(
  "Path", 
  "$env:Path;C:\Tools\ask", 
  [System.EnvironmentVariableTarget]::User
)
```

**Verify PATH:**
```powershell
$env:Path -split ';' | Select-String ask
```

### AWS Credentials on Windows

AWS CLI works identically in PowerShell:
```powershell
aws configure
```

Credentials stored in: `%USERPROFILE%\.aws\credentials`

### Editors

Popular markdown editors for Windows:
- **VS Code** (recommended): `code session.md`
- **Notepad++**: `notepad++ session.md`
- **Typora**: Visual markdown editing
- **Obsidian**: Knowledge management focus

---

## Philosophy

- **Markdown files are central** — They are the source of truth
- **The tool disappears** — It should feel like thinking, not using software
- **Explicit over magic** — You understand what's happening
- **Knowledge replaces features** — The tool amplifies what you know

---

## Advanced Usage

### Chaining Multiple References

```markdown

Analyze the entire request pipeline:

[[cmd/chat.go]]
[[internal/bedrock/stream.go]]
[[internal/session/stream.go]]

Where are potential bottlenecks?
```

### Selective Expansion

```markdown

Compare just the core logic:

[[pkg/core.go]]

Ignore tests and generated code.
```

### Multi-Project Context

```markdown

How does this project compare to:

[[../other-project/README.md]]
[[../other-project/architecture/]]

What architectural decisions differ?
```

---

## Troubleshooting

### Session Parsing Errors

**"No human turn found in session.md":**
- Ensure you have at least one `# [1] Human` section
- Check for typos in turn headers
- Turn numbers must be sequential

**"Turn has no content":**
- Add your thoughts between the turn header and next section
- Empty turns cannot be processed

### File Reference Issues

**"Cannot find 'file.txt' referenced in turn 3":**
- Verify file path is relative to `session.md` location
- Check file exists: `ls file.txt`
- Use `[[./file.txt]]` for current directory

**"No matching files in directory":**
- Directory may not contain included file types
- Check exclusion patterns: `ask cfg show`
- Use `[[dir/**/]]` to force recursive search

### Model Access

**"Model requires additional setup":**
- Model may not be available in your AWS region
- Try: `ask cfg models` to see available options
- Switch to a different model: `ask cfg model opus`

### Performance

**"Request timeout":**
- Increase timeout: `ask cfg timeout 10m`
- Reduce `max-tokens`: `ask cfg max-tokens 16000`
- Split large requests into smaller turns

**Large token counts:**
- Enable filtering: `ask cfg filter enable on`
- Strip headers: `ask cfg filter headers on`
- Use selective file references instead of entire directories

