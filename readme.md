# ASCII-Art-Web

## Description

ASCII-Art-Web is the web version of the [ascii-art](https://01.nextera.education/api/content/root/01-edu_module/content/ascii-art) CLI project. It runs a Go HTTP server with a browser-based GUI: a user types text into a form, picks a banner style (`standard`, `shadow`, or `thinkertoy`), and the server returns the same ASCII-art rendering the CLI tool produces — displayed directly on the page instead of printed to a terminal.

The rendering algorithm itself is unchanged from the CLI project. What changed is how input arrives (an HTML form instead of `os.Args`) and how output leaves (an HTML page instead of stdout).

## Authors

- Cody

## Refactor: from CLI to web-ready

Before adding any HTTP code, the original `ascii-art` `main.go` was refactored so its core logic could be reused by a web handler instead of a CLI entrypoint. Two functions changed signature; nothing about the rendering algorithm changed:

| Function | Before | After | Why |
|---|---|---|---|
| `render` | `func render(input string, banner []string) error` — printed each line directly with `fmt.Println` | `func render(input string, banner []string) (string, error)` — builds the full output in a `strings.Builder` and returns it | A web handler can't print to stdout; it needs a string to hand to `html/template` |
| `printAscii` | `func printAscii(text string, banner []string) error` — printed each of the 8 rows with `fmt.Println` | `func printAscii(text string, banner []string) string` — returns the 8 rows as one string (each row still newline-terminated) | Same reason — output needs to be a value, not a side effect |
| `run` | Called `render` and returned its `error` directly | Calls `render`, gets `(output, err)` back, and does a single `fmt.Print(output)` to preserve identical CLI behavior | Keeps the CLI usable and byte-for-byte identical while the underlying functions become reusable |

`bannerFilename` and `loadBanner` were left completely untouched — they already returned values rather than performing side effects, so there was nothing to change.

This refactor was done *before* writing any `net/http` code, so that the HTTP handlers could simply call `render(...)` and get a string back, with zero coupling to how that string is eventually displayed.

## Project structure

```
ascii-art-web/
├── go.mod
├── main.go
├── README.MD
├── standard.txt
├── shadow.txt
├── thinkertoy.txt
├── static/
│   └── style.css
└── templates/
    └── index.html
```

Templates live in `templates/` per the subject's requirement. Banner `.txt` files stay at the project root, matching the relative-path lookup already used by `loadBanner`.

## Usage

Requires Go (standard library only — no external packages).

```bash
git clone <your-repo-url>
cd ascii-art-web
go run .
```

Then open `http://localhost:8080` in a browser.

1. Type text into the text field.
2. Choose a banner style (`standard` is selected by default).
3. Click **Generate**.

The result is appended to the same page, below the form, so the form stays populated with what you just submitted.

## Implementation details

### Banner file format

Each banner file (`standard.txt`, `shadow.txt`, `thinkertoy.txt`) encodes all 95 printable ASCII characters (codes 32–126), 9 lines each: 8 lines of character art, 1 blank separator line. `loadBanner` reads the file, normalizes line endings, and validates the line count matches `95 × 9` exactly — a mismatch means a corrupted or wrong-format banner file, so an error is returned rather than continuing.

### Character lookup

For a character `ch`, its 8-line block starts at index:

```
(int(ch) - firstPrintable) * linesPerChar
```

where `firstPrintable = 32` and `linesPerChar = 9`. `render` walks the input, and `printAscii` builds each of the 8 output rows by concatenating the corresponding line from every character's block, left to right — so a horizontal row across the final banner is built by reading vertically-corresponding lines from each character's own 8-line block.

### Web server (`net/http` + `html/template`)

- **`GET /`** — serves the form via a single parsed template (`templates/index.html`), executed once at startup and reused across requests. Any path other than `/` returns `404`; any method other than `GET` returns `405`.
- **`POST /ascii-art`** — reads `text` and `banner` from the submitted form (`r.FormValue`), then runs the exact same `bannerFilename` → `loadBanner` → `render` pipeline as the CLI version. The result (or an error message) is passed back into the same template and re-rendered, so the page updates in place rather than navigating elsewhere.
- **Status codes** are chosen based on *why* something failed: empty input or an unrecognized banner name is the client's fault (`400`); a missing or malformed banner file on disk is a `404`; anything the template itself fails to execute is a `500`.
- `html/template` (not `text/template`) is used specifically because it auto-escapes the text the user submits before it's echoed back into the page, preventing injected HTML/script from the input field.

## Notes on approach

- **One template, appended results**: rather than a separate results page, `/ascii-art` re-renders `templates/index.html` with the result filled in. This keeps the form and its last output visible together, and avoids needing a second template file or a redirect.
- **No third-party packages**: only the Go standard library is used (`net/http`, `html/template`, `os`, `strings`, `fmt`, `log`), per the subject's constraints.
