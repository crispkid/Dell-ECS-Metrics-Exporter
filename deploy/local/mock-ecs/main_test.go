package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLoadFixturesAndAuthenticatedRoutes(t *testing.T) {
	fixtures, err := loadFixtures(filepath.Join("..", "..", "..", "testdata", "ecs"))
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(fixtures, "test-token")

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/vdc/nodes", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	loginRequest := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginRequest.SetBasicAuth("monitor", "synthetic")
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK || login.Header().Get(tokenHeader) != "test-token" {
		t.Fatalf("login status=%d token=%q", login.Code, login.Header().Get(tokenHeader))
	}

	nodesRequest := httptest.NewRequest(http.MethodGet, "/vdc/nodes", nil)
	nodesRequest.Header.Set(tokenHeader, "test-token")
	nodes := httptest.NewRecorder()
	handler.ServeHTTP(nodes, nodesRequest)
	if nodes.Code != http.StatusOK ||
		!strings.Contains(nodes.Body.String(), "3.6.2.6.123456.synthetic") {
		t.Fatalf("nodes status=%d body=%s", nodes.Code, nodes.Body.String())
	}
}

func TestFluxRouting(t *testing.T) {
	fixtures := fixtureSet{
		nodeCPU:              []byte(`{"kind":"cpu"}`),
		nodeMemory:           []byte(`{"kind":"memory"}`),
		nodeNetwork:          []byte(`{"kind":"network"}`),
		nodeDisk:             []byte(`{"kind":"disk"}`),
		vdcCorePerformance:   []byte(`{"kind":"vdc-core"}`),
		vdcLatency:           []byte(`{"kind":"vdc-latency"}`),
		namespacePerformance: []byte(`{"kind":"namespace"}`),
	}
	tests := []struct {
		name   string
		query  string
		status int
		body   string
	}{
		{"cpu", `r._measurement == "cpu"`, http.StatusOK, `"cpu"`},
		{"memory", `r._measurement == "mem"`, http.StatusOK, `"memory"`},
		{"network", `r._measurement == "net"`, http.StatusOK, `"network"`},
		{"disk", `r._measurement == "disk"`, http.StatusOK, `"disk"`},
		{"vdc core", `r._measurement == "cq_performance_throughput"`, http.StatusOK, `"vdc-core"`},
		{"vdc latency", `r._measurement == "cq_performance_latency"`, http.StatusOK, `"vdc-latency"`},
		{"namespace", `r._measurement == "cq_performance_transaction_ns"`, http.StatusOK, `"namespace"`},
		{"unsupported", `from(bucket: "unknown")`, http.StatusBadRequest, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/flux/api/external/v2/query",
				strings.NewReader(`{"query":`+strconv.Quote(test.query)+`}`),
			)
			body, status := routeFluxFixture(request, fixtures)
			if status != test.status || (test.body != "" && !strings.Contains(string(body), test.body)) {
				t.Fatalf("status=%d body=%s", status, body)
			}
		})
	}
}
