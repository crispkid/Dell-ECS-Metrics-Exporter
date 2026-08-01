package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

const tokenHeader = "X-SDS-AUTH-TOKEN"

type fixtureSet struct {
	nodes                []byte
	clusterHealth        []byte
	clusterCapacity      []byte
	namespaces           []byte
	namespaceQuota       []byte
	namespaceBilling     []byte
	buckets              []byte
	bucketQuota          []byte
	bucketBilling        []byte
	bucketBillingBatch   []byte
	nodeCPU              []byte
	nodeMemory           []byte
	nodeNetwork          []byte
	nodeDisk             []byte
	vdcCorePerformance   []byte
	vdcLatency           []byte
	namespacePerformance []byte
	replicationGroup     []byte
	replicationLink      []byte
}

type fixtureFile struct {
	target    *[]byte
	directory string
	name      string
}

func main() {
	listen := flag.String("listen", "127.0.0.1:4443", "HTTPS listen address")
	fixtures := flag.String("fixtures", "testdata/ecs", "fixture root directory")
	flag.Parse()

	data, err := loadFixtures(*fixtures)
	if err != nil {
		log.Fatal(err)
	}
	token, err := randomToken()
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}

	server := httptest.NewUnstartedServer(newHandler(data, token))
	server.Listener = listener
	server.StartTLS()
	log.Printf("synthetic Dell ECS HTTPS API listening at %s", server.URL)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	server.Close()
}

func loadFixtures(root string) (fixtureSet, error) {
	read := func(directory, name string) ([]byte, error) {
		path := filepath.Join(root, directory, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read fixture %s: %w", path, err)
		}
		return content, nil
	}
	var result fixtureSet
	values := []fixtureFile{
		{&result.nodes, "ecs-3.6", "nodes.json"},
		{&result.clusterHealth, "common", "localzone-health.json"},
		{&result.clusterCapacity, "common", "capacity.json"},
		{&result.namespaces, "common", "namespaces.json"},
		{&result.namespaceQuota, "common", "namespace-quota-configured.json"},
		{&result.namespaceBilling, "common", "namespace-billing-info.json"},
		{&result.buckets, "common", "buckets.json"},
		{&result.bucketQuota, "common", "bucket-quota-configured.json"},
		{&result.bucketBilling, "common", "bucket-billing-info.json"},
		{&result.bucketBillingBatch, "common", "bucket-billing-batch.json"},
		{&result.nodeCPU, "common", "flux-node-cpu.json"},
		{&result.nodeMemory, "common", "flux-node-memory.json"},
		{&result.nodeNetwork, "common", "flux-node-network.json"},
		{&result.nodeDisk, "common", "flux-node-disk.json"},
		{&result.vdcCorePerformance, "common", "flux-vdc-core-performance.json"},
		{&result.vdcLatency, "common", "flux-vdc-latency.json"},
		{&result.namespacePerformance, "common", "flux-namespace-performance.json"},
		{&result.replicationGroup, "common", "replication-group.json"},
		{&result.replicationLink, "common", "rg-link.json"},
	}
	for _, value := range values {
		content, err := read(value.directory, value.name)
		if err != nil {
			return fixtureSet{}, err
		}
		*value.target = content
	}
	return result, nil
}

func randomToken() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func newHandler(fixtures fixtureSet, token string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/login" {
			handleLogin(writer, request, token)
			return
		}
		if request.Header.Get(tokenHeader) != token {
			http.Error(writer, "authentication required", http.StatusUnauthorized)
			return
		}
		if request.URL.Path == "/logout" {
			if request.Method != http.MethodGet {
				http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
			return
		}

		content, status := routeFixture(request, fixtures)
		if status != http.StatusOK {
			http.Error(writer, http.StatusText(status), status)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(content)
	})
}

func handleLogin(writer http.ResponseWriter, request *http.Request, token string) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username, password, ok := request.BasicAuth()
	if !ok || strings.TrimSpace(username) == "" || password == "" {
		http.Error(writer, "authentication required", http.StatusUnauthorized)
		return
	}
	writer.Header().Set(tokenHeader, token)
	writer.WriteHeader(http.StatusOK)
}

func routeFixture(request *http.Request, fixtures fixtureSet) ([]byte, int) {
	path := request.URL.Path
	switch {
	case request.Method == http.MethodGet && path == "/vdc/nodes":
		return fixtures.nodes, http.StatusOK
	case request.Method == http.MethodGet && path == "/user/whoami":
		return []byte(`{"user":{"common_name":"monitor","roles":{"role":["SYSTEM_MONITOR"]}}}`), http.StatusOK
	case request.Method == http.MethodGet && path == "/dashboard/zones/localzone":
		return fixtures.clusterHealth, http.StatusOK
	case request.Method == http.MethodGet && path == "/object/capacity":
		return fixtures.clusterCapacity, http.StatusOK
	case request.Method == http.MethodGet && path == "/dashboard/zones/localzone/nodes":
		return []byte(`{"node":[{"nodeid":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","healthStatus":"Good","services":[{"name":"blobsvc","status":"running"}],"processes":[{"name":"fabric-lifecycle","status":"healthy"}]},{"nodeid":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","healthStatus":"Good"}]}`), http.StatusOK
	case request.Method == http.MethodGet && path == "/object/namespaces":
		return fixtures.namespaces, http.StatusOK
	case request.Method == http.MethodGet &&
		path == "/object/namespaces/namespace/namespace-a/quota":
		return fixtures.namespaceQuota, http.StatusOK
	case request.Method == http.MethodGet &&
		path == "/object/billing/namespace/namespace-a/info":
		return fixtures.namespaceBilling, http.StatusOK
	case request.Method == http.MethodGet && path == "/object/bucket":
		if request.URL.Query().Get("namespace") != "namespace-a" {
			return nil, http.StatusBadRequest
		}
		return fixtures.buckets, http.StatusOK
	case request.Method == http.MethodGet &&
		strings.HasPrefix(path, "/object/bucket/") &&
		strings.HasSuffix(path, "/quota"):
		return fixtures.bucketQuota, http.StatusOK
	case request.Method == http.MethodPost &&
		path == "/object/billing/buckets/namespace-a/info":
		return fixtures.bucketBillingBatch, http.StatusOK
	case request.Method == http.MethodGet &&
		strings.HasPrefix(path, "/object/billing/buckets/namespace-a/") &&
		strings.HasSuffix(path, "/info"):
		return fixtures.bucketBilling, http.StatusOK
	case request.Method == http.MethodPost && path == "/flux/api/external/v2/query":
		return routeFluxFixture(request, fixtures)
	case request.Method == http.MethodGet && path == "/dashboard/replicationgroups/rg-a":
		return fixtures.replicationGroup, http.StatusOK
	case request.Method == http.MethodGet && path == "/dashboard/rglinks/rg-link-a":
		return fixtures.replicationLink, http.StatusOK
	default:
		return nil, http.StatusNotFound
	}
}

func routeFluxFixture(request *http.Request, fixtures fixtureSet) ([]byte, int) {
	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		return nil, http.StatusBadRequest
	}
	switch {
	case strings.Contains(body.Query, `"cq_performance_transaction_ns"`):
		return fixtures.namespacePerformance, http.StatusOK
	case strings.Contains(body.Query, `"cq_performance_latency"`):
		return fixtures.vdcLatency, http.StatusOK
	case strings.Contains(body.Query, `"cq_performance_throughput"`):
		return fixtures.vdcCorePerformance, http.StatusOK
	case strings.Contains(body.Query, `r._measurement == "cpu"`):
		return fixtures.nodeCPU, http.StatusOK
	case strings.Contains(body.Query, `r._measurement == "mem"`):
		return fixtures.nodeMemory, http.StatusOK
	case strings.Contains(body.Query, `r._measurement == "net"`):
		return fixtures.nodeNetwork, http.StatusOK
	case strings.Contains(body.Query, `r._measurement == "disk"`):
		return fixtures.nodeDisk, http.StatusOK
	}
	return nil, http.StatusBadRequest
}
