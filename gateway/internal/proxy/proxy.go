package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"gateway/internal/router"
)

func NewHandler(table *router.Table) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matched, remainder := table.Match(r.URL.Path)
		if matched == nil {
			http.NotFound(w, r)
			return
		}

		target, err := url.Parse(matched.Upstream)
		if err != nil {
			http.Error(w, "bad upstream", http.StatusBadGateway)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error: upstream=%s err=%v", matched.Upstream, err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}

		r.URL.Path = remainder
		r.URL.Host = target.Host
		r.URL.Scheme = target.Scheme
		r.Host = target.Host

		proxy.ServeHTTP(w, r)
	}
}
