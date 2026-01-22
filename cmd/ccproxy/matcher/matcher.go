package matcher

// Matcher determines if a command should be proxied to the host
type Matcher interface {
	// Matches returns true if the command should be proxied to the host
	Matches(fullCommand string) bool
}
