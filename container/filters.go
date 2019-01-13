package container

// A Filter is a prototype for a function that can be used to filter the
// results from a call to the ListContainers() method on the Client.
type Filter func(FilterableContainer) bool

// A FilterableContainer is the interface which is used to filter
// containers.
type FilterableContainer interface {
	Name() string
	IsWatchtower() bool
	WatchtowerTag() (string, bool)
}

// WatchtowerContainersFilter filters only watchtower containers
func BuildWatchtowerContainersFilter(tag string) Filter {
	return func(c FilterableContainer) bool {
		containerTag, hasTag := c.WatchtowerTag()
		return hasTag && containerTag == tag && c.IsWatchtower()
	}
}

// BuildTagFilter creates the needed filter of containers
func BuildTagFilter(tag string) Filter {
	return func(c FilterableContainer) bool {
		containerTag, hasTag := c.WatchtowerTag()
		return hasTag && containerTag == tag
	}
}
