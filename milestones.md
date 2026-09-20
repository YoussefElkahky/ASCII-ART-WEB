# MILESTONES.md — ASCII-Art → ASCII-Art-Web

This document walks through every milestone of turning the `ascii-art` CLI project into `ascii-art-web`. Each milestone follows the same structure: **why it exists**, **the concepts you need first**, **the code, explained line by line**, and a **checkpoint** to confirm before moving on.

---

# Milestone 1 — Confirm the CLI logic works standalone

## Why this milestone exists

Everything that follows builds *on top of* your existing `ascii-art` code. If there's a bug in the core rendering logic, you want to find it now — while it's just a CLI program you can test in seconds — not three milestones from now, buried under HTTP handlers and templates where it's much harder to tell whether a wrong result comes from the algorithm or from the web layer around it.

## What to check

Run the CLI through every case it's supposed to handle:

```bash
go run . "hello"
go run . "hello" standard
go run . "hello" shadow
go run . "hello" thinkertoy
go run . "hello\nworld" standard
go run . "" standard
go run . "café" standard
go run . "hello" bogus
```

- The first four should print correctly aligned ASCII art in each banner style.
- The `\n` case should split into two separate blocks of art, one per line.
- Empty input should do nothing (exit quietly, per your original `main`'s `if input == "" { return }`).
- `café` should fail with your "unsupported character" error, since `é` is outside printable ASCII.
- `bogus` should fail with your "unknown banner style" error.

## Checkpoint

Every case above should behave exactly as you expect *before* you touch a single line of code for the web version. This is your safety net — if something breaks later, you'll know with confidence it's new code, not something that was already broken.

---

# Milestone 2 — Refactor `render`/`printAscii` to return strings

## Why this milestone exists

Your CLI's `render` and `printAscii` currently *print* — they call `fmt.Println`, and that's the end of the story; the text goes straight to the terminal and is gone. That works for a CLI because the terminal *is* the only place the output needs to go.

A web server can't do that. When a browser sends a request, your Go handler needs to build the entire HTML response as **data in memory**, then hand that whole thing to `net/http` to send back over the network. There's no "terminal" to print into. So the output of your rendering logic has to become something you can *hold onto and pass around* — a `string` — rather than something that's printed and immediately discarded.

This is the distinction between:
- **Side effects** — a function does something to the outside world (prints to a terminal, writes to a file) and gives nothing back.
- **Return values** — a function computes something and hands it back to whoever called it, who decides what to do with it.

This milestone converts `render`/`printAscii` from the first kind to the second. Nothing about the *math* changes — only how the result leaves the function.

## The concepts you need first

**Multiple return values `(string, error)`.** A very common Go pattern: the first value is "the thing you wanted," the second is "did it go wrong." Callers check `err` before trusting the first value.

**`strings.Builder`.** If you built output with plain concatenation (`output += ...`), it would *work*, but inefficiently: Go strings are immutable, so every `+=` allocates a brand-new string and copies everything that came before it. For a short loop this doesn't matter, but the cost grows roughly with the square of the output size. `strings.Builder` maintains one growable internal buffer and appends into it directly:

```go
var b strings.Builder
b.WriteString("hello")
b.WriteString(" world")
s := b.String()   // "hello world"
b.Reset()         // empty it, ready to reuse
```

`Reset()` is why you'll see it used once per row inside `printAscii` — one `Builder` is reused and cleared 8 times, rather than creating 8 separate ones.

## The code

```go
func render(input string, banner []string) (string, error) {

	for _, ch := range input {
		if ch == '\n' {
			continue
		}
		if ch < firstPrintable || ch > lastPrintable {
			return "", fmt.Errorf("unsupported character: %q", ch)
		}
	}

	trimmed := input
	if strings.Trim(input, "\n") == "" {
		trimmed = strings.TrimSuffix(input, "\n")
	}

	lines := strings.Split(trimmed, "\n")

	var output strings.Builder

	for _, line := range lines {
		if line == "" {
			output.WriteString("\n")
			continue
		}
		output.WriteString(printAscii(line, banner))
	}

	return output.String(), nil
}

func printAscii(text string, banner []string) string {

	var builder strings.Builder
	var result strings.Builder

	for row := 0; row < charHeight; row++ {
		builder.Reset()
		for _, ch := range text {
			index := (int(ch)-firstPrintable)*linesPerChar + row
			builder.WriteString(banner[index])
		}
		result.WriteString(builder.String())
		result.WriteString("\n")
	}

	return result.String()
}
```

Walking through what changed:

- **Validation loop** — unchanged logic, but `return "", fmt.Errorf(...)` now returns an empty string alongside the error, since the function's first return slot is a `string` and every `return` statement has to give it something. Standard Go idiom: on error, return the zero value and let `err != nil` signal not to use it.
- **`var output strings.Builder`** — declared once, before the loop, since *all* lines' output accumulates into the same builder across the whole function.
- **`output.WriteString("\n")` for blank lines / `output.WriteString(printAscii(line, banner))` for real lines** — previously `fmt.Println()` and a side-effecting call to `printAscii`; now both cases *append into* `output` instead of printing.
- **`return output.String(), nil`** — converts the builder into a final string and returns it. This is the one place the entire rendered output becomes a single string value any caller can use.
- **`printAscii`'s two builders** — `builder` is scratch space for *one row at a time* (reset every iteration); `result` accumulates *all 8 finished rows* (never reset). The core lookup math — `(int(ch)-firstPrintable)*linesPerChar + row` — is completely untouched.
- **`result.WriteString(builder.String()); result.WriteString("\n")`** — previously `fmt.Println(builder.String())`; now the finished row is appended into `result`, with the newline added explicitly since nothing is "printing a line" anymore.

Update `run` to print the string once, at the very end:

```go
output, err := render(input, banner)
if err != nil {
	return err
}
fmt.Print(output)
```

Note `fmt.Print`, not `fmt.Println` — every row inside `output` already ends with its own `\n` (added in `printAscii`), so `Println`'s extra one would leave a stray blank line.

## Checkpoint

Run every case from Milestone 1 again. Output must be **byte-for-byte identical** to before this refactor. If it's not, the bug is almost always a missing or extra `\n` somewhere in the string-building — fix it now, before adding HTTP code on top.

---

# Milestone 3 — Set up the project skeleton

## Why this milestone exists

Before writing server code, the physical project structure needs to be right — the subject requires templates to live in a specific place (`templates/`), and Go needs a module (`go.mod`) to manage anything beyond a single file. This step is pure setup, no logic yet.

## The commands

```bash
mkdir ascii-art-web
cd ascii-art-web
go mod init ascii-art-web

mkdir templates static
cp /path/to/old/ascii-art/{standard,shadow,thinkertoy}.txt .
cp /path/to/old/ascii-art/main.go .
```

Resulting tree:
```
ascii-art-web/
├── go.mod
├── main.go
├── standard.txt
├── shadow.txt
├── thinkertoy.txt
├── static/
└── templates/
```

- `go mod init ascii-art-web` creates `go.mod`, declaring this folder as a Go module named `ascii-art-web`. Without it, `go run .` still technically works for a single file, but you lose proper dependency tracking and some tooling assumes a module exists.
- `templates/` and `static/` are reserved now, empty, so nothing later has to be restructured.

## Checkpoint

`tree` (or `ls -R`) should match the structure above, and `go run .` (with your CLI's `main.go` still in place, pre-refactor-to-web) should still work exactly as it did before, confirming the module itself is set up correctly.

---

# Milestone 4 — Build the bare HTTP server

## Why this milestone exists

Up to now your code is a library of functions that compute things — nothing has ever *listened* for anything. A web server sits there indefinitely, waiting for incoming connections, and runs code per-request. This milestone builds that "sit and wait" machinery with almost no real logic in it yet, so you can prove the skeleton works before building the actual form on top of it. Scope is deliberately narrow: `GET /` responds with *something*, everything else returns `404`.

## The concepts you need first

**`net/http`'s three pieces:** a router (`ServeMux`) that matches a request's path to code, a handler function with the fixed shape `func(w http.ResponseWriter, r *http.Request)`, and a listener that opens a network port and feeds connections to the router.

**`http.ResponseWriter` and `*http.Request`.** Every handler gets exactly these two, always in this order. `r` is everything about the incoming request (method, path, headers) — you read from it. `w` is what you write to, to build the response — anything written to it becomes the response body.

**Why `"/"` is a catch-all, not an exact match.** `mux.HandleFunc("/", ...)` in Go's default `ServeMux` matches *every* path not claimed by something more specific — not just the literal `/`. So `/foo`, `/anything` all land in the same handler unless you check `r.URL.Path` yourself and return `404` for anything that isn't exactly `/`.

**Why `html/template`, not `text/template`.** `html/template` auto-escapes values inserted into the page — a user typing `<script>` into a form later can't inject working HTML/JS. You're not echoing user input yet in this milestone, but wiring it up now avoids a foot-gun later.

**Why the template is parsed once, at startup.** `template.ParseFiles(...)` reads and compiles the file into memory. Doing this per-request would mean re-reading the same unchanging file from disk on every hit — pure waste. Parsed once into a package-level variable (`var tmpl *template.Template`), every request reuses it.

## The code

```go
package main

import (
	"html/template"
	"log"
	"net/http"
)

var tmpl *template.Template

func main() {
	var err error
	tmpl, err = template.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatal("could not load template: ", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tmpl.Execute(w, nil)
}
```

- `template.ParseFiles(...)` returns `(*template.Template, error)`. A non-nil `err` (missing file, bad syntax) triggers `log.Fatal` — there's no reasonable way to serve pages without the template, so failing loudly at startup beats starting a server that errors on every request.
- `http.NewServeMux()` — an explicit router, kept self-contained to this file, rather than depending on Go's global default mux.
- `mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))` — read inside-out: `http.Dir("static")` treats the folder as servable; `http.FileServer` wraps it into a handler; `http.StripPrefix` removes `/static/` from the URL before the lookup, so `/static/style.css` correctly maps to the file `static/style.css` (not `static/static/style.css`).
- `http.ListenAndServe(":8080", mux)` **blocks** — it runs forever, serving requests, and only returns if something goes wrong *before* it could start (e.g. the port's already in use). `log.Fatal(...)` around it means: if that happens, print it and exit.
- `handleIndex`'s two guard clauses (`r.URL.Path != "/"`, `r.Method != http.MethodGet`) fix the catch-all gotcha and reject non-GET requests, both via `http.Error` — a shortcut that sets the status code and writes a plain-text body in one call.

Placeholder `templates/index.html`:
```html
<!DOCTYPE html>
<html>
<head><title>ASCII Art Web</title></head>
<body><h1>ASCII Art Web</h1></body>
</html>
```

## Checkpoint

`go run .`, confirm `listening on http://localhost:8080` prints. Then: `GET /` shows the placeholder page with `200`; `GET /foo` returns `404`; `curl -i -X POST http://localhost:8080/` returns `405`.

---

# Milestone 5 — Build the form and wire up `GET /`

## Why this milestone exists

Milestone 4's page is static and dumb — it can't remember what the user typed, can't show which banner is selected, can't display anything back. This milestone builds the real page and a shared data structure (`PageData`) that lets **one** template render three different situations — first load, success, failure — without three separate templates.

## The concepts you need first

**Why one shared struct, not three.** The template is the same HTML file regardless of situation (the form is always visible; only what's below it changes). Defining one struct with every field any scenario might need, and leaving unused fields at their zero value (`""`), lets `{{if .Error}}` / `{{if .Result}}` simply not render when those fields are empty — an empty string is "falsy" in a template `{{if}}`.

**Template actions used here:**
- `{{.FieldName}}` — inserts a field's value.
- `{{if .FieldName}} ... {{end}}` — renders content only if the field is non-empty.
- `{{if eq .Banner "shadow"}} ... {{end}}` — `eq` is the built-in equality function; templates don't support `==`.
- `{{if or (eq .Banner "standard") (eq .Banner "")}} ... {{end}}` — `or` combines conditions; the "or empty" half matters because on first load `Banner` hasn't been set to anything yet.

**Why exported struct fields are required.** `html/template` needs to read struct fields via reflection from outside your package — it can only see **exported** (capitalized) fields. A lowercase field like `text` would be invisible to `{{.text}}`.

## The code

```go
type PageData struct {
	Text   string
	Banner string
	Result string
	Error  string
}
```

Updated `handleIndex`:

```go
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := tmpl.Execute(w, PageData{Banner: "standard"}); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
}
```

`PageData{Banner: "standard"}` — Go struct literal syntax, setting one field by name and leaving the rest (`Text`, `Result`, `Error`) at `""`. `Execute`'s error is now checked (unlike Milestone 4's bare version) and mapped to `500` if it fails.

`templates/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>ASCII Art Web</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <h1>ASCII Art Web</h1>

    <form method="POST" action="/ascii-art">
        <textarea name="text" rows="3" placeholder="Type something...">{{.Text}}</textarea>

        <fieldset>
            <label>
                <input type="radio" name="banner" value="standard"
                    {{if or (eq .Banner "standard") (eq .Banner "")}}checked{{end}}>
                Standard
            </label>
            <label>
                <input type="radio" name="banner" value="shadow"
                    {{if eq .Banner "shadow"}}checked{{end}}>
                Shadow
            </label>
            <label>
                <input type="radio" name="banner" value="thinkertoy"
                    {{if eq .Banner "thinkertoy"}}checked{{end}}>
                Thinkertoy
            </label>
        </fieldset>

        <button type="submit">Generate</button>
    </form>

    {{if .Error}}
        <p class="error">{{.Error}}</p>
    {{end}}

    {{if .Result}}
        <h2>Result</h2>
        <pre>{{.Result}}</pre>
    {{end}}
</body>
</html>
```

- `<form method="POST" action="/ascii-art">` — submitting sends a `POST` to `/ascii-art`. Nothing handles it yet (that's Milestone 6), so submitting at this stage will error — expected.
- `<textarea name="text" ...>{{.Text}}</textarea>` — `name="text"` is the identifier the server will later read via `r.FormValue("text")`; it must match exactly. `{{.Text}}` pre-fills it, letting typed input survive a failed submission once Milestone 6 exists.
- All three radios share `name="banner"` — that's what makes them mutually exclusive as a group, and it's the field name read later. `{{if ...}}checked{{end}}` conditionally inserts the literal `checked` attribute, which is what makes a radio appear pre-selected.
- The `{{if .Error}}` / `{{if .Result}}` blocks are skipped entirely on first load since both fields are empty — this is the mechanism letting one template serve every situation.

**Auto-escaping note:** since this uses `html/template`, `{{.Text}}` doesn't dump the raw string in — a user typing `<b>hi</b>` renders as the literal text `<b>hi</b>` (escaped), not as bold or executable HTML. Automatic, no extra code needed — the payoff of choosing `html/template` back in Milestone 4.

## Checkpoint

Load `/`: textarea empty, "Standard" pre-selected, no error/result HTML present at all (check via view-source, not just visually). Click through the radios and confirm only one is selectable at a time. Submitting is fine if it doesn't work yet — just confirm it doesn't crash the server.

---

# Milestone 6 — Implement `POST /ascii-art` end to end

## Why this milestone exists

This is where everything connects: a submitted form turns into a call to `bannerFilename → loadBanner → render` — your original CLI pipeline, unmodified — and the result or a specific failure gets shown back with the right status code. The core discipline: for every failure, ask **whose fault is this?**

- Client's fault (empty text, unrecognized banner name, unsupported character) → `400`
- Server's fault (a banner `.txt` file is missing/corrupt) → `404`
- Unexpected (template execution fails) → `500`

## The concepts you need first

**`r.ParseForm()` / `r.FormValue(...)`.** A submitted `<form method="POST">` encodes fields into the request body (`text=hello&banner=standard`). `r.ParseForm()` parses that body, returning an error if it's malformed. `r.FormValue("text")` looks up a field by name after parsing — the names must match the `name="..."` attributes from Milestone 5's HTML exactly.

**Why `data := PageData{...}` is built before any validation.** So no matter which failure path fires, the response can echo back exactly what the user typed/selected, rather than resetting the form to blank on every mistake.

**`w.WriteHeader(...)` vs `http.Error(...)`.** `http.Error` is a plain-text shortcut, used for the method check. But failures here need the **full HTML page** re-rendered with the error inside the styled `<p class="error">` block and the form still populated — `http.Error` can't do that. Instead: `w.WriteHeader(code)` sets the status, then `tmpl.Execute(w, data)` writes the actual body. `WriteHeader` must be called before any body content and only once per response.

**Why the success path skips `WriteHeader`.** If you never call it explicitly, Go sends `200 OK` automatically the first time you write to `w` — which happens inside `tmpl.Execute`.

## The code

Register the route in `main`, alongside the existing `"/"` line:

```go
mux.HandleFunc("/ascii-art", handleAsciiArt)
```

```go
func handleAsciiArt(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "400 bad request", http.StatusBadRequest)
		return
	}

	text := r.FormValue("text")
	bannerStyle := r.FormValue("banner")

	data := PageData{Text: text, Banner: bannerStyle}

	if text == "" {
		data.Error = "please enter some text"
		w.WriteHeader(http.StatusBadRequest)
		tmpl.Execute(w, data)
		return
	}

	filename, err := bannerFilename(bannerStyle)
	if err != nil {
		data.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
		tmpl.Execute(w, data)
		return
	}

	bannerLines, err := loadBanner(filename)
	if err != nil {
		data.Error = "banner file not found"
		w.WriteHeader(http.StatusNotFound)
		tmpl.Execute(w, data)
		return
	}

	input := strings.ReplaceAll(text, `\n`, "\n")

	output, err := render(input, bannerLines)
	if err != nil {
		data.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
		tmpl.Execute(w, data)
		return
	}

	data.Result = output
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}
}
```

- **Method check** — same pattern as `handleIndex`; only `POST` makes sense here.
- **`r.ParseForm()`** — a failure here is malformed at the HTTP level, so a plain `400` with no form to echo back is appropriate.
- **`data := PageData{Text: text, Banner: bannerStyle}`** — built immediately, before validation.
- **Empty text check** — `400`, `data.Error` set on the already-built struct (mutation, not a new struct), which lights up `{{if .Error}}`.
- **`bannerFilename(bannerStyle)`** — your original, untouched CLI function. A user can't normally trigger this through the radio buttons, but someone bypassing the form (e.g. via `curl`) could send an arbitrary value — still `400`, since it's a bad request regardless of how it was sent. `err.Error()` converts the Go error to its string message for display.
- **`loadBanner(filename)`** — the one `404` branch. `loadBanner` fails when the `.txt` file is missing or malformed *on the server* — not something the form-filler did wrong, hence `404` instead of `400`. The message shown is a generic `"banner file not found"`, **not** `err.Error()` — the real error includes the server's file path, which shouldn't be exposed to users.
- **`strings.ReplaceAll(text, `\n`, "\n")`** — carried over unchanged from the original CLI's `run`, converting a literal `\n` typed by the user into an actual newline.
- **`render(input, bannerLines)`** — your Milestone 2 function, unchanged since. Its only failure mode now is an unsupported character — `400`, and `err.Error()` is safe to show since that message only contains the offending character.
- **Success path** — `data.Result = output` fills the field that makes `{{if .Result}}` render. No explicit `WriteHeader` (implicit `200`). `Execute`'s error is checked one final time — a failure this late is unexpected, hence `500`.

Every failure branch follows the same three-step shape (`data.Error = ...`; `w.WriteHeader(code)`; `tmpl.Execute(w, data)`) — only the message and code differ.

## Checkpoint

```bash
curl -i -X POST -d "text=&banner=standard" http://localhost:8080/ascii-art        # expect 400
curl -i -X POST -d "text=hi&banner=standard" http://localhost:8080/ascii-art      # expect 200
curl -i -X POST -d "text=hi&banner=shadow" http://localhost:8080/ascii-art        # expect 200
curl -i -X POST -d "text=hi&banner=thinkertoy" http://localhost:8080/ascii-art    # expect 200
curl -i -X POST -d "text=hi&banner=bogus" http://localhost:8080/ascii-art         # expect 400
curl -i -X POST -d "text=café&banner=standard" http://localhost:8080/ascii-art    # expect 400
```

Check the actual `HTTP/1.1 ___` status line each time, not just whether the response "looks okay." In the browser, confirm text and banner selection survive a failed submission instead of resetting.

---

# Milestone 7 — Style the page and handle display details

## Why this milestone exists

The handler (M6) now returns a technically correct HTML page — but rendered in a browser, the alignment would very likely be broken. Not a Go bug: HTML by default collapses whitespace and multiple spaces, and uses a proportional font where different characters have different widths — both of which actively destroy ASCII art, which depends on fixed-width columns lining up exactly.

## The concepts you need first

**Why `<pre>` specifically.** One of the only HTML elements where whitespace is preserved exactly as written — every space and newline rendered literally, instead of collapsed the way `<div>` or `<p>` would.

**Why `font-family: monospace` matters here.** A monospace font gives every character the exact same width. Your Milestone 2 algorithm implicitly assumes this: it concatenates fixed-width character blocks side by side, trusting column N in one block lines up with column N in the next. A proportional font breaks that visually even though the string itself is correct — a bug invisible in Go code, only visible by actually looking at the page.

**Why static CSS, not inline styles.** `/static/` was already registered as a route back in Milestone 4 — any file dropped into `static/` becomes servable at `/static/<filename>` automatically. This milestone is where that finally gets used.

## The code

`static/style.css`:

```css
body {
    font-family: monospace, sans-serif;
    max-width: 700px;
    margin: 2rem auto;
    padding: 0 1rem;
}

textarea {
    font-family: inherit;
    font-size: 1rem;
    padding: 0.5rem;
    resize: vertical;
    width: 100%;
}

fieldset {
    border: 1px solid #ccc;
    border-radius: 4px;
}

.error {
    color: #b00020;
    font-weight: bold;
}

.ascii-result {
    background: #f5f5f5;
    padding: 1rem;
    overflow-x: auto;
    border-radius: 4px;
    line-height: 1.1;
}
```

- `font-family: monospace, sans-serif` — `font-family` is a list, tried in order. `monospace` is a generic keyword the browser maps to a fixed-width font; `sans-serif` is a fallback that essentially never gets used, since `monospace` support is universal.
- `.ascii-result { overflow-x: auto; ... }` — a long input string produces wide art (many characters × many columns each). Without this, a wide `<pre>` would force the *entire page* to scroll horizontally. With it, only the art gets its own horizontal scrollbar when needed. `line-height: 1.1` tightens the default vertical gap between rows, which the art doesn't need.
- `.error { color: #b00020; font-weight: bold; }` — purely visual distinction so error messages don't blend into normal page text.

Updated `templates/index.html`:

```html
<head>
    <meta charset="UTF-8">
    <title>ASCII Art Web</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
```

```html
{{if .Result}}
    <h2>Result</h2>
    <pre class="ascii-result">{{.Result}}</pre>
{{end}}
```

`<link rel="stylesheet" href="/static/style.css">` — a standard HTML tag, no template syntax. `href="/static/style.css"` is an absolute path (starts with `/`), resolving correctly from any page. The request hits the `/static/` route from Milestone 4, gets stripped down to `style.css`, and `FileServer` serves the file from your `static/` folder.

Only change to the result block: adding `class="ascii-result"` so the CSS rule above applies.

**A note on escaping:** wrapping `{{.Result}}` in `<pre>` doesn't change `html/template`'s auto-escaping — that mechanism is unrelated to how whitespace renders. `<pre>` only affects whitespace; escaping still applies exactly as it did in Milestone 5.

## Checkpoint

Check the Network tab for `style.css` loading with `200` (a `404` usually means it's not actually at `static/style.css`, or you're not running `go run .` from the project root). Generate art with a reasonably long string in each banner style and **visually confirm** every row lines up — this is a milestone where nothing errors in the terminal even when it's broken, so the visual check isn't optional. Resize the browser narrower and confirm wide art scrolls within its own box rather than breaking the page layout. Trigger an error and confirm it's visibly red/bold.

---

# Milestone 8 — Test edge cases, then document

## Why this milestone exists

Manual testing here means deliberately trying to break your own server the way a grader or reviewer would — not just confirming the happy path once. Testing outside the browser (via `curl`) matters because a client could send a POST that skips your HTML form's constraints entirely (e.g. the radio buttons only ever send one of three valid values — `curl` doesn't have to respect that).

## What to run

```bash
# empty text
curl -i -X POST -d "text=&banner=standard" http://localhost:8080/ascii-art

# invalid banner name, bypassing the radio buttons entirely
curl -i -X POST -d "text=hi&banner=bogus" http://localhost:8080/ascii-art

# unsupported character
curl -i -X POST -d "text=café&banner=standard" http://localhost:8080/ascii-art

# unknown route
curl -i http://localhost:8080/nonsense

# wrong method on each route
curl -i -X POST http://localhost:8080/
curl -i http://localhost:8080/ascii-art
```

For each, check the `HTTP/1.1 ___` status line and confirm it matches Milestone 6's mapping (`400` for bad client input, `404` for missing server resources or unknown routes, `405` for wrong methods).

- `curl -i` sends the raw HTTP status line along with the response body, so you're verifying the actual status code rather than trusting how the browser *looks* like it behaved.

## Documentation

Once every case behaves correctly, write `README.MD` at the project root with:

- **Description** — what the project does, one or two sentences.
- **Authors** — you (and anyone else who worked on it).
- **Usage** — how to run it (`go run .`, then visit `localhost:8080`).
- **Implementation details** — the banner file format (95 characters × 9 lines), the character-lookup math (`(ch-32)*9 + row`), and how the web layer maps failures to status codes.

Document the finished, tested behavior — not what you *intend* to build — since this is the last step, after everything above is confirmed working.

## Checkpoint

Every `curl` case above returns the expected status code, the browser flow (fill form → submit → see result or error) works end to end for all three banner styles, and `README.MD` exists and accurately describes what's actually implemented.

---

# Summary: the shape of the whole project

```
CLI functions (bannerFilename, loadBanner, render, printAscii)
    — unchanged from Milestone 2 onward —
        ↓
handleAsciiArt reads the form, calls the same functions,
maps each possible failure to the right status code
        ↓
tmpl.Execute renders the result (or error) into the
same page the form lives on, via PageData
```

The web layer (Milestones 4–7) is entirely new code, but it's a thin shell around rendering logic that was finalized in Milestone 2 and never touched again. That's the core idea behind doing the refactor first: once `render` returns a string instead of printing, everything after it is just plumbing.