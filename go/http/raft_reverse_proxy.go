package http

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/proxysql/golib/log"

	"github.com/proxysql/orchestrator/go/raft"
)

// raftReverseProxyMiddleware returns chi middleware that proxies requests to
// the raft leader when the current node is not the leader.
func raftReverseProxyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !orcraft.IsRaftEnabled() {
			next.ServeHTTP(w, r)
			return
		}
		if orcraft.IsLeader() {
			next.ServeHTTP(w, r)
			return
		}
		if orcraft.GetLeader() == "" {
			next.ServeHTTP(w, r)
			return
		}
		if orcraft.LeaderURI.IsThisLeaderURI() {
			next.ServeHTTP(w, r)
			return
		}
		u, err := url.Parse(orcraft.LeaderURI.Get())
		if err != nil {
			log.Errore(err)
			next.ServeHTTP(w, r)
			return
		}
		r.Header.Del("Accept-Encoding")
		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.Transport, err = orcraft.GetRaftHttpTransport()
		if err != nil {
			log.Errore(err)
			next.ServeHTTP(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}
