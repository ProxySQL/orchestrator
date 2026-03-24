/*
   Copyright 2015 Shlomi Noach, courtesy Booking.com

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package http

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/inst"
	"github.com/proxysql/orchestrator/go/os"
	"github.com/proxysql/orchestrator/go/process"
	"github.com/proxysql/orchestrator/go/raft"
)

// contextKey is used for storing values in context.
type contextKey string

const userContextKey contextKey = "auth-user"

// getUserFromContext retrieves the authenticated user from the request context.
func getUserFromContext(r *http.Request) string {
	if user, ok := r.Context().Value(userContextKey).(string); ok {
		return user
	}
	return ""
}

func getProxyAuthUser(req *http.Request) string {
	for _, user := range req.Header[config.Config.AuthUserHeader] {
		return user
	}
	return ""
}

// isAuthorizedForAction checks req to see whether authenticated user has write-privileges.
// This depends on configured authentication method.
func isAuthorizedForAction(req *http.Request) bool {
	if config.Config.ReadOnly {
		return false
	}

	if orcraft.IsRaftEnabled() && !orcraft.IsLeader() {
		// A raft member that is not a leader is unauthorized.
		return false
	}

	user := getUserFromContext(req)
	switch strings.ToLower(config.Config.AuthenticationMethod) {
	case "basic":
		{
			// The mere fact we're here means the user has passed authentication
			return true
		}
	case "multi":
		{
			if user == "readonly" {
				// read only
				return false
			}
			// passed authentication ==> writeable
			return true
		}
	case "proxy":
		{
			authUser := getProxyAuthUser(req)
			for _, configPowerAuthUser := range config.Config.PowerAuthUsers {
				if configPowerAuthUser == "*" || configPowerAuthUser == authUser {
					return true
				}
			}
			// check the user's group is one of those listed here
			if len(config.Config.PowerAuthGroups) > 0 && os.UserInGroups(authUser, config.Config.PowerAuthGroups) {
				return true
			}
			return false
		}
	case "token":
		{
			cookie, err := req.Cookie("access-token")
			if err != nil {
				return false
			}

			publicToken := strings.Split(cookie.Value, ":")[0]
			secretToken := strings.Split(cookie.Value, ":")[1]
			result, _ := process.TokenIsValid(publicToken, secretToken)
			return result
		}
	case "oauth":
		{
			return false
		}
	default:
		{
			// Default: no authentication method
			return true
		}
	}
}

func authenticateToken(publicToken string, resp http.ResponseWriter) error {
	secretToken, err := process.AcquireAccessToken(publicToken)
	if err != nil {
		return err
	}
	cookieValue := fmt.Sprintf("%s:%s", publicToken, secretToken)
	cookie := &http.Cookie{Name: "access-token", Value: cookieValue, Path: "/"}
	http.SetCookie(resp, cookie)
	return nil
}

// getUserId returns the authenticated user id, if available, depending on authentication method.
func getUserId(req *http.Request) string {
	if config.Config.ReadOnly {
		return ""
	}

	user := getUserFromContext(req)
	switch strings.ToLower(config.Config.AuthenticationMethod) {
	case "basic":
		{
			return user
		}
	case "multi":
		{
			return user
		}
	case "proxy":
		{
			return getProxyAuthUser(req)
		}
	case "token":
		{
			return ""
		}
	default:
		{
			return ""
		}
	}
}

// getClusterHintFromRequest extracts the cluster hint from chi URL params.
func getClusterHintFromRequest(r *http.Request) string {
	if v := chi.URLParam(r, "clusterHint"); v != "" {
		return v
	}
	if v := chi.URLParam(r, "clusterName"); v != "" {
		return v
	}
	if host := chi.URLParam(r, "host"); host != "" {
		if port := chi.URLParam(r, "port"); port != "" {
			return fmt.Sprintf("%s:%s", host, port)
		}
	}
	return ""
}

// getClusterNameIfExistsFromRequest returns a cluster name by request hint, or an empty cluster name
// if no hint is given
func getClusterNameIfExistsFromRequest(r *http.Request) (clusterName string, err error) {
	if clusterHint := getClusterHintFromRequest(r); clusterHint == "" {
		return "", nil
	} else {
		return figureClusterName(clusterHint)
	}
}

// figureClusterName is a convenience function to get a cluster name from hints
func figureClusterName(hint string) (clusterName string, err error) {
	if hint == "" {
		return "", fmt.Errorf("Unable to determine cluster name by empty hint")
	}
	instanceKey, _ := inst.ParseRawInstanceKey(hint)
	return inst.FigureClusterName(hint, instanceKey, nil)
}

// BasicAuthMiddleware returns a middleware that performs HTTP Basic Authentication.
func BasicAuthMiddleware(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MultiAuthMiddleware returns a middleware that performs HTTP Basic Authentication
// with a special "readonly" user.
func MultiAuthMiddleware(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if u == "readonly" {
				// Will be treated as "read-only"
				ctx := context.WithValue(r.Context(), userContextKey, u)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			if subtle.ConstantTimeCompare([]byte(u), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
