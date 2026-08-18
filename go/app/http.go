/*
   Copyright 2014 Outbrain Inc.

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

package app

import (
	"net"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/proxysql/orchestrator/go/agent"
	"github.com/proxysql/orchestrator/go/collection"
	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/http"
	"github.com/proxysql/orchestrator/go/inst"
	"github.com/proxysql/orchestrator/go/logic"
	ometrics "github.com/proxysql/orchestrator/go/metrics"
	"github.com/proxysql/orchestrator/go/process"
	"github.com/proxysql/orchestrator/go/ssl"

	"github.com/proxysql/golib/log"
)

const discoveryMetricsName = "DISCOVERY_METRICS"

var sslPEMPassword []byte
var agentSSLPEMPassword []byte
var discoveryMetrics *collection.Collection

func revalidateStaticAssets(next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		next.ServeHTTP(w, r)
	})
}

// Http starts serving
func Http(continuousDiscovery bool) {
	promptForSSLPasswords()
	ometrics.InitPrometheus()
	process.ContinuousRegistration(string(process.OrchestratorExecutionHttpMode), "")

	if config.Config.ServeAgentsHttp {
		go agentsHttp()
	}
	standardHttp(continuousDiscovery)
}

// Iterate over the private keys and get passwords for them
// Don't prompt for a password a second time if the files are the same
func promptForSSLPasswords() {
	if ssl.IsEncryptedPEM(config.Config.SSLPrivateKeyFile) {
		sslPEMPassword = ssl.GetPEMPassword(config.Config.SSLPrivateKeyFile)
	}
	if ssl.IsEncryptedPEM(config.Config.AgentSSLPrivateKeyFile) {
		if config.Config.AgentSSLPrivateKeyFile == config.Config.SSLPrivateKeyFile {
			agentSSLPEMPassword = sslPEMPassword
		} else {
			agentSSLPEMPassword = ssl.GetPEMPassword(config.Config.AgentSSLPrivateKeyFile)
		}
	}
}

// standardHttp starts serving HTTP or HTTPS (api/web) requests, to be used by normal clients
func standardHttp(continuousDiscovery bool) {
	router := chi.NewRouter()

	// Middleware
	router.Use(chimiddleware.Logger)
	router.Use(chimiddleware.Recoverer)
	router.Use(chimiddleware.Compress(5))

	switch strings.ToLower(config.Config.AuthenticationMethod) {
	case "basic":
		{
			if config.Config.HTTPAuthUser == "" {
				// Still allowed; may be disallowed in future versions
				_ = log.Warning("AuthenticationMethod is configured as 'basic' but HTTPAuthUser undefined. Running without authentication.")
			}
			router.Use(http.BasicAuthMiddleware(config.Config.HTTPAuthUser, config.Config.HTTPAuthPassword))
		}
	case "multi":
		{
			if config.Config.HTTPAuthUser == "" {
				// Still allowed; may be disallowed in future versions
				log.Fatal("AuthenticationMethod is configured as 'multi' but HTTPAuthUser undefined")
			}

			router.Use(http.MultiAuthMiddleware(config.Config.HTTPAuthUser, config.Config.HTTPAuthPassword))
		}
	default:
		{
			// No authentication method - nothing to add
		}
	}

	// Static file serving
	prefix := config.Config.URLPrefix
	fileServer := revalidateStaticAssets(nethttp.FileServer(nethttp.Dir("resources/public")))
	if prefix != "" {
		router.Handle(prefix+"/*", nethttp.StripPrefix(prefix, fileServer))
	} else {
		router.Handle("/*", fileServer)
	}

	if config.Config.UseMutualTLS {
		router.Use(ssl.VerifyOUsMiddleware(config.Config.SSLValidOUs))
	}

	inst.SetMaintenanceOwner(process.ThisHostname)

	if continuousDiscovery {
		// start to expire metric collection info
		discoveryMetrics = collection.CreateOrReturnCollection(discoveryMetricsName)
		discoveryMetrics.SetExpirePeriod(time.Duration(config.Config.DiscoveryCollectionRetentionSeconds) * time.Second)

		log.Info("Starting Discovery")
		go logic.ContinuousDiscovery()
	}

	log.Info("Registering endpoints")
	http.API.URLPrefix = config.Config.URLPrefix
	http.Web.URLPrefix = config.Config.URLPrefix
	http.API.RegisterRequests(router)
	http.RegisterV2Routes(router)
	http.Web.RegisterRequests(router)

	// Serve
	if config.Config.ListenSocket != "" {
		log.Infof("Starting HTTP listener on unix socket %v", config.Config.ListenSocket)
		unixListener, err := net.Listen("unix", config.Config.ListenSocket)
		if err != nil {
			log.Fatale(err)
		}
		defer func() { _ = unixListener.Close() }()
		if err := nethttp.Serve(unixListener, router); err != nil {
			log.Fatale(err)
		}
	} else if config.Config.UseSSL {
		log.Info("Starting HTTPS listener")
		tlsConfig, err := ssl.NewTLSConfig(config.Config.SSLCAFile, config.Config.UseMutualTLS)
		if err != nil {
			log.Fatale(err)
		}
		tlsConfig.InsecureSkipVerify = config.Config.SSLSkipVerify
		if err = ssl.AppendKeyPairWithPassword(tlsConfig, config.Config.SSLCertFile, config.Config.SSLPrivateKeyFile, sslPEMPassword); err != nil {
			log.Fatale(err)
		}
		if err = ssl.ListenAndServeTLS(config.Config.ListenAddress, router, tlsConfig); err != nil {
			log.Fatale(err)
		}
	} else {
		log.Infof("Starting HTTP listener on %+v", config.Config.ListenAddress)
		if err := nethttp.ListenAndServe(config.Config.ListenAddress, router); err != nil {
			log.Fatale(err)
		}
	}
	log.Info("Web server started")
}

// agentsHttp starts serving agents HTTP or HTTPS API requests
func agentsHttp() {
	router := chi.NewRouter()
	router.Use(chimiddleware.Compress(5))

	if config.Config.AgentsUseMutualTLS {
		router.Use(ssl.VerifyOUsMiddleware(config.Config.AgentSSLValidOUs))
	}

	log.Info("Starting agents listener")

	agent.InitHttpClient()
	go logic.ContinuousAgentsPoll()

	http.AgentsAPI.URLPrefix = config.Config.URLPrefix
	http.AgentsAPI.RegisterRequests(router)

	// Serve
	if config.Config.AgentsUseSSL {
		log.Info("Starting agent HTTPS listener")
		tlsConfig, err := ssl.NewTLSConfig(config.Config.AgentSSLCAFile, config.Config.AgentsUseMutualTLS)
		if err != nil {
			log.Fatale(err)
		}
		tlsConfig.InsecureSkipVerify = config.Config.AgentSSLSkipVerify
		if err = ssl.AppendKeyPairWithPassword(tlsConfig, config.Config.AgentSSLCertFile, config.Config.AgentSSLPrivateKeyFile, agentSSLPEMPassword); err != nil {
			log.Fatale(err)
		}
		if err = ssl.ListenAndServeTLS(config.Config.AgentsServerPort, router, tlsConfig); err != nil {
			log.Fatale(err)
		}
	} else {
		log.Info("Starting agent HTTP listener")
		if err := nethttp.ListenAndServe(config.Config.AgentsServerPort, router); err != nil {
			log.Fatale(err)
		}
	}
	log.Info("Agent server started")
}
