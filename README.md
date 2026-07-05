# notion-ssg

`notion-ssg` is an opinionated Notion Static Site Generator. It reads a Notion page subtree, converts supported Notion blocks into portable static HTML, and writes a complete site to a local output directory.

The tool is designed for personal sites, project notes, lightweight blogs, and small knowledge bases where Notion is the authoring environment and the generated site is the public artifact.

## Status

Early implementation. The CLI, theme format, Notion API coverage, and generated markup are expected to evolve before the first stable release.

## Install

Download the binary for your platform from GitHub Releases:

- macOS: `notion-ssg_darwin_amd64` or `notion-ssg_darwin_arm64`
- Linux: `notion-ssg_linux_amd64` or `notion-ssg_linux_arm64`
- Windows: `notion-ssg_windows_amd64.exe`

Then place it somewhere on your `PATH`.

```sh
chmod +x notion-ssg_darwin_arm64
mv notion-ssg_darwin_arm64 /usr/local/bin/notion-ssg
notion-ssg --help
```

Release binaries are built by GitHub Actions for Windows, macOS, and Linux.

## Prerequisites

You need a Notion Personal Access Token with access to the workspace content you want to publish.

- Guide: https://developers.notion.com/guides/get-started/personal-access-tokens
- Token console: https://app.notion.com/developers/tokens

The integration must be connected to the relevant pages. For workspace-wide root discovery, use a token from a workspace owner or an integration with sufficient access to search and read the target tree.

Never commit tokens. Prefer `$NOTION_PAT` in your shell profile or a local `.env` file that is ignored by git.

## Usage

### Local Usage

Generate a site from a Notion page into a local static-site directory:

```sh
export NOTION_PAT=secret_xxx
notion-ssg --nav-root "My Site" --theme minimal --output ./public
```

Equivalent explicit-token form:

```sh
notion-ssg --notion-pat secret_xxx --nav-root "My Site" --theme themes/minimal.yaml --output ./public
```

`--notion-pat` takes precedence over `$NOTION_PAT`.

### CI

This pattern runs `notion-ssg` hourly, commits generated changes, and pushes them back to the static-site repository. It assumes:

- `NOTION_PAT` is configured as a repository secret.
- `DEPLOY_KEY` is configured as a repository secret containing a private deploy key with push access.
- `notion-ssg` is available from GitHub.

```yaml
name: Generate site

on:
  schedule:
    - cron: "0 * * * *"
  workflow_dispatch:

jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ssh-key: ${{ secrets.DEPLOY_KEY }}

      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - run: go install github.com/chasemduffin/notion-ssg/cmd/notion-ssg@main

      - run: notion-ssg --nav-root "My Site" --theme minimal --output .
        env:
          NOTION_PAT: ${{ secrets.NOTION_PAT }}

      - name: Commit generated changes
        env:
          GIT_AUTHOR_NAME: site-generator
          GIT_AUTHOR_EMAIL: site-generator@example.com
          GIT_COMMITTER_NAME: site-generator
          GIT_COMMITTER_EMAIL: site-generator@example.com
        run: |
          git add -A
          git diff --cached --quiet || git commit -m "chore: update generated site"
          git push
```

## CLI

```text
Usage of notion-ssg:
  --nav-root string
        Notion page title to use as the site root
  --notion-pat string
        Notion Personal Access Token; defaults to $NOTION_PAT
  --output string
        Output directory for generated static files
  --theme string
        Built-in theme name or path to theme.yaml (default "minimal")
```

## Theme Configuration

A theme can be a built-in name or a YAML file. Starter themes live in `themes/`.

```yaml
name: minimal
mode: spa
font_family: system
accent: "#000000"
max_width: 76rem
flair: terminal-rule
```

`mode` controls navigation behavior:

- `static`: generate normal static pages with full page loads.
- `spa`: generate normal static pages plus a small progressive-enhancement script that intercepts internal navigation, fetches the next page, and swaps content with smooth transitions.

All generated themes must be responsive across phones, tablets, laptops, desktop monitors, narrow windows, high-DPI screens, keyboard navigation, and reduced-motion preferences.

## Modules

Modules are named pages under the Notion root that can opt into custom generation logic while still using the common theme and navigation system.

- `blog`: renders database rows as post subpages, excludes rows whose `Status` is not `Published`, renders tag columns as color-coded filter chips, and supports sortable date columns.
- `gallery`: currently a stub module. It creates the navigation target and page shell but does not emit gallery-specific content yet.
- Any other root child page uses the default module and renders supported Notion blocks and databases without module-specific filtering.

## Notion Content Support

Support is intentionally pragmatic. The generator favors clear, stable public output over exact Notion parity.

| Notion feature | Support | Output behavior |
| --- | --- | --- |
| Paragraphs | Yes | HTML paragraphs with rich text annotations |
| Headings 1-3 | Yes | Semantic heading tags |
| Bulleted lists | Yes | Simple list items |
| Numbered lists | Yes | Simple ordered list items |
| To-do blocks | Yes | Disabled checkboxes |
| Quotes | Yes | `blockquote` |
| Callouts | Yes | Styled aside with text |
| Code blocks | Yes | `pre > code` with language class |
| Dividers | Yes | `hr` |
| Images | Partial | External/file URLs rendered as images when accessible |
| Files | Partial | Download links when accessible |
| Links and bookmarks | Yes | External links |
| Child pages | Yes | Generated as child pages and navigation links |
| Child databases | Partial | Database rows rendered as index tables and subpages |
| Columns | Simplified | Responsive column groups |
| Synced blocks | No | Deferred |
| Advanced embeds | No | Deferred unless reducible to a link |
| Formulas and rollups | Partial | Plain display only when returned by the API |
| Comments | No | Not included in public output |
| Private mentions | Partial | Plain text when available; no private resolution |
| Unsupported third-party embeds | No | Rendered as links or omitted |

## Testing

The project uses Go's built-in `testing` package.

The test strategy is based on mockable boundaries:

- config parsing tests for flag/env precedence;
- fixture-backed Notion API tests;
- block-to-HTML conversion tests for supported block types;
- generated-site tests using deterministic fake Notion trees;
- golden-style assertions for important HTML/CSS/JS output.

Run:

```sh
go test ./...
```

## Output Safety

The output directory is treated as generated content. Hand-written files may be replaced. Use a dedicated output directory or keep the target repository under version control so generated changes can be reviewed before commit.

## Contributing

Contributions from humans and agents are welcome.

Good contributions are small, test-backed, and easy to review. Agentic contributions should include the prompt/context used, avoid committing secrets or generated noise, and keep diffs focused enough for human review.

Before opening a PR:

```sh
go test ./...
gofmt -w .
```

## License

BSD 3-Clause. See `LICENSE`.
