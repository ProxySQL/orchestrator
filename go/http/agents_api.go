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

package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/proxysql/orchestrator/go/agent"
	"github.com/proxysql/orchestrator/go/attributes"
)

type HttpAgentsAPI struct {
	URLPrefix string
}

var AgentsAPI HttpAgentsAPI = HttpAgentsAPI{}

// SubmitAgent registers an agent. It is initiated by an agent to register itself.
func (this *HttpAgentsAPI) SubmitAgent(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(chi.URLParam(r, "port"))
	if err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: err.Error()})
		return
	}

	output, err := agent.SubmitAgent(chi.URLParam(r, "host"), port, chi.URLParam(r, "token"))
	if err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: err.Error()})
		return
	}
	renderJSON(w, 200, output)
}

// SetHostAttribute is a utility method that allows per-host key-value store.
func (this *HttpAgentsAPI) SetHostAttribute(w http.ResponseWriter, r *http.Request) {
	err := attributes.SetHostAttributes(chi.URLParam(r, "host"), chi.URLParam(r, "attrVame"), chi.URLParam(r, "attrValue"))

	if err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: fmt.Sprintf("%+v", err)})
		return
	}

	renderJSON(w, 200, (err == nil))
}

// GetHostAttributeByAttributeName returns a host attribute
func (this *HttpAgentsAPI) GetHostAttributeByAttributeName(w http.ResponseWriter, r *http.Request) {

	output, err := attributes.GetHostAttributesByAttribute(chi.URLParam(r, "attr"), r.URL.Query().Get("valueMatch"))

	if err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: fmt.Sprintf("%+v", err)})
		return
	}

	renderJSON(w, 200, output)
}

// AgentsHosts provides list of agent host names
func (this *HttpAgentsAPI) AgentsHosts(w http.ResponseWriter, r *http.Request) {
	agents, err := agent.ReadAgents()
	hostnames := []string{}
	for _, agent := range agents {
		hostnames = append(hostnames, agent.Hostname)
	}

	if err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: fmt.Sprintf("%+v", err)})
		return
	}

	if r.URL.Query().Get("format") == "txt" {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Join(hostnames, "\n")))
	} else {
		renderJSON(w, 200, hostnames)
	}
}

// AgentsInstances provides list of assumed MySQL instances (host:port)
func (this *HttpAgentsAPI) AgentsInstances(w http.ResponseWriter, r *http.Request) {
	agents, err := agent.ReadAgents()
	hostnames := []string{}
	for _, agent := range agents {
		hostnames = append(hostnames, fmt.Sprintf("%s:%d", agent.Hostname, agent.MySQLPort))
	}

	if err != nil {
		renderJSON(w, 200, &APIResponse{Code: ERROR, Message: fmt.Sprintf("%+v", err)})
		return
	}

	if r.URL.Query().Get("format") == "txt" {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Join(hostnames, "\n")))
	} else {
		renderJSON(w, 200, hostnames)
	}
}

func (this *HttpAgentsAPI) AgentPing(w http.ResponseWriter, r *http.Request) {
	renderJSON(w, 200, "OK")
}

// RegisterRequests makes for the de-facto list of known API calls
func (this *HttpAgentsAPI) RegisterRequests(router chi.Router) {
	router.Get(this.URLPrefix+"/api/submit-agent/{host}/{port}/{token}", this.SubmitAgent)
	router.Get(this.URLPrefix+"/api/host-attribute/{host}/{attrVame}/{attrValue}", this.SetHostAttribute)
	router.Get(this.URLPrefix+"/api/host-attribute/attr/{attr}/", this.GetHostAttributeByAttributeName)
	router.Get(this.URLPrefix+"/api/agents-hosts", this.AgentsHosts)
	router.Get(this.URLPrefix+"/api/agents-instances", this.AgentsInstances)
	router.Get(this.URLPrefix+"/api/agent-ping", this.AgentPing)
}
