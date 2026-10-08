package tui

// Options configures Run.
type Options struct {
	Getenv func(string) string // colour detection; nil -> os.Getenv
}
