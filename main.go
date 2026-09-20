package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	charHeight     = 8
	linesPerChar   = charHeight + 1 
	firstPrintable = 32
	lastPrintable  = 126
)

// PageData is passed into templates/index.html on every render.
type PageData struct {
	Text   string // last submitted text, so the form can re-populate it
	Banner string // last submitted banner style, so the right radio stays checked
	Result string // rendered ASCII art, empty until a successful POST
	Error  string // error message to display, empty when there is none
}

var tmpl *template.Template

func main() {

	var err error
	tmpl, err = template.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatal("could not load template: ", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/ascii-art", handleAsciiArt)
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

	renderPage(w, PageData{Banner: "standard"})
}

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
		renderPage(w, data)
		return
	}

	filename, err := bannerFilename(bannerStyle)
	if err != nil {
		data.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
		renderPage(w, data)
		return
	}

	banner, err := loadBanner(filename)
	if err != nil {
		// Missing/invalid banner file: treat as not found.
		data.Error = "banner file not found"
		w.WriteHeader(http.StatusNotFound)
		renderPage(w, data)
		return
	}

	input := strings.ReplaceAll(text, `\n`, "\n")

	output, err := render(input, banner)
	if err != nil {
		data.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
		renderPage(w, data)
		return
	}

	data.Result = output
	renderPage(w, data)
}

// renderPage executes the shared template. If it fails after headers may
// already be written, we can only log it -- but if it happens because the
// template itself is missing, that's a 500 (a runtime/setup fault, not a
// bad request).
func renderPage(w http.ResponseWriter, data PageData) {
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
	}
}

func bannerFilename(style string) (string, error) {
	switch style {
	case "", "standard":
		return "standard.txt", nil
	case "shadow":
		return "shadow.txt", nil
	case "thinkertoy":
		return "thinkertoy.txt", nil
	default:
		return "", fmt.Errorf("unknown banner style %q (expected standard, shadow or thinkertoy)", style)
	}
}

func loadBanner(filename string) ([]string, error) {

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("could not open banner file %s: %s", filename, err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")

	// (126 - 32 + 1) printable ASCII characters * 9 lines per character.
	expectedLines := (lastPrintable - firstPrintable + 1) * linesPerChar

	banner := strings.Split(strings.TrimPrefix(content, "\n"), "\n")

	if len(banner) != expectedLines {
		return nil, fmt.Errorf(
			"invalid banner file %s: expected %d lines, got %d",
			filename,
			expectedLines,
			len(banner),
		)
	}

	return banner, nil
}

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