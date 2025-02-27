package watcher

// Option represents a configuration function that modifies watcher object.
type Option func(*watcher)

var (
	defaultOptions = []Option{}
)
