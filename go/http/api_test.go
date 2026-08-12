package http

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/proxysql/golib/log"
	test "github.com/proxysql/golib/tests"
	"github.com/proxysql/orchestrator/go/config"
	"github.com/proxysql/orchestrator/go/inst"
)

func init() {
	config.Config.HostnameResolveMethod = "none"
	config.MarkConfigurationLoaded()
	log.SetLevel(log.ERROR)
}

func TestGetSynonymPath(t *testing.T) {
	api := HttpAPI{}

	{
		path := "relocate-slaves"
		synonym := api.getSynonymPath(path)
		test.S(t).ExpectEquals(synonym, "relocate-replicas")
	}
	{
		path := "relocate-slaves/:host/:port"
		synonym := api.getSynonymPath(path)
		test.S(t).ExpectEquals(synonym, "relocate-replicas/:host/:port")
	}
}

func TestKnownPaths(t *testing.T) {
	router := chi.NewRouter()
	api := HttpAPI{}

	api.RegisterRequests(router)

	pathsMap := make(map[string]bool)
	for _, path := range registeredPaths {
		pathBase := strings.Split(path, "/")[0]
		pathsMap[pathBase] = true
	}
	test.S(t).ExpectTrue(pathsMap["health"])
	test.S(t).ExpectTrue(pathsMap["lb-check"])
	test.S(t).ExpectTrue(pathsMap["relocate"])
	test.S(t).ExpectTrue(pathsMap["relocate-slaves"])

	for path, synonym := range apiSynonyms {
		test.S(t).ExpectTrue(pathsMap[path])
		test.S(t).ExpectTrue(pathsMap[synonym])
	}
}

func TestMaintenanceBegunResponseReturnsCreatedMaintenanceKey(t *testing.T) {
	response := maintenanceBegunResponse(inst.InstanceKey{Hostname: "mysql2", Port: 3306}, 42)

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Code    string
		Message string
		Details int64
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Code != "OK" {
		t.Fatalf("expected OK code, got %q", decoded.Code)
	}
	if decoded.Message != "Maintenance begun: mysql2:3306" {
		t.Fatalf("expected existing message to be preserved, got %q", decoded.Message)
	}
	if decoded.Details != 42 {
		t.Fatalf("expected created maintenance key 42 in Details, got %d", decoded.Details)
	}
}
