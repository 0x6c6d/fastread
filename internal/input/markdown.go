package input

// loadMarkdown is a stub: Markdown is read as plain text until the Markdown loader lands.
func loadMarkdown(b []byte) ([]string, error) {
	return ParseText(b), nil
}
