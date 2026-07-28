package router

import "strings"

type Route struct {
	Path        string
	Upstream    string
	KeepPrefix  bool // true=不剥离前缀，默认剥离
}

type Table struct {
	routes []Route
}

func NewTable(routes []Route) *Table {
	return &Table{routes: routes}
}

// Match 最长前缀匹配。默认剥离 Path 前缀。
func (t *Table) Match(requestPath string) (*Route, string) {
	var best *Route
	bestLen := 0

	for i := range t.routes {
		r := &t.routes[i]
		if strings.HasPrefix(requestPath, r.Path) {
			if len(r.Path) > bestLen {
				best = r
				bestLen = len(r.Path)
			}
		}
	}

	if best == nil {
		return nil, ""
	}

	if best.KeepPrefix {
		return best, requestPath
	}

	remainder := strings.TrimPrefix(requestPath, best.Path)
	if remainder == "" {
		remainder = "/"
	}
	return best, remainder
}
