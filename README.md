# Ask

A thought preservation and amplification system using AWS Bedrock Claude and markdown files.

## Philosophy

Ask treats markdown files as a source of truth. The tool orchestrates conversations but the markdown session is the primary artifact. Your thoughts, enriched with AI comprehension, become preserved knowledge.

## Installation

```bash
go install github.com/rana/ask@latest
```

## Usage

### Start a Session

```bash
ask init
```

Creates `session.md` in the current directory.

### Add Your Thoughts

Edit `session.md` with your preferred editor:

```markdown
## [1] Human

I'm exploring distributed systems. 

[[architecture.md]]
[[patterns.md]]

What patterns emerge from these designs?
```

### Process the Session

```bash
ask
```

Ask will:
1. Expand your `[[file]]` references into numbered sections
2. Send to Claude via AWS Bedrock  
3. Append the AI response
4. Show token counts for awareness

### Continue the Conversation

Add a new human turn to `session.md`:

```markdown
## [3] Human

Let's dive deeper into consensus mechanisms.

[[consensus.md]]
```

Run `ask` again to continue.

## Configuration

Requires AWS credentials with access to Bedrock Claude:

```bash
aws configure
```

## Design Principles

- **Markdown files are central** - They are the source of truth
- **The tool disappears** - It should feel like thinking, not like using software
- **Explicit over magic** - You understand what's happening
- **Knowledge replaces features** - The tool amplifies what you know

## License

MIT
