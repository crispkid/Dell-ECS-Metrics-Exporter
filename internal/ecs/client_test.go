package ecs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dell-ecs-metrics-exporter/internal/config"
)

func TestClientAuthenticationReauthenticationRetryAndLogout(t *testing.T) {
	t.Parallel()
	var logins, retryCalls, logoutCalls atomic.Int32
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			username, password, ok := request.BasicAuth()
			if !ok || username != "monitor" || password != "secret" {
				http.Error(writer, "bad basic auth", http.StatusUnauthorized)
				return
			}
			count := logins.Add(1)
			writer.Header().Set(tokenHeader, "token-"+string(rune('0'+count)))
			writer.WriteHeader(http.StatusOK)
		case "/data":
			if request.Header.Get(tokenHeader) == "token-1" {
				http.Error(writer, "expired", http.StatusUnauthorized)
				return
			}
			_, _ = writer.Write([]byte(`{"value":"ok"}`))
		case "/retry":
			if retryCalls.Add(1) == 1 {
				http.Error(writer, "temporary", http.StatusServiceUnavailable)
				return
			}
			_, _ = writer.Write([]byte(`{"retried":true}`))
		case "/post":
			if request.Method != http.MethodPost || request.URL.Query().Get("q") != "value" ||
				request.Header.Get("Content-Type") != "application/json" {
				http.Error(writer, "bad request", http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(request.Body)
			if string(body) != `{"input":"value"}` {
				http.Error(writer, "bad body", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"posted":true}`))
		case "/logout":
			logoutCalls.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	})

	observer := &recordingObserver{}
	client := newTestClient(t, handler, observer)
	var result map[string]any
	if err := client.GetJSON(context.Background(), "data", "/data", nil, &result); err != nil ||
		result["value"] != "ok" || logins.Load() != 2 {
		t.Fatalf("reauth result=%v logins=%d err=%v", result, logins.Load(), err)
	}
	data, err := client.GetBytes(context.Background(), "retry", "/retry", nil)
	if err != nil || !bytes.Contains(data, []byte(`"retried":true`)) || retryCalls.Load() != 2 {
		t.Fatalf("retry data=%s calls=%d err=%v", data, retryCalls.Load(), err)
	}
	result = nil
	if err := client.PostJSON(
		context.Background(), "post", "/post", url.Values{"q": []string{"value"}},
		map[string]string{"input": "value"}, &result,
	); err != nil || result["posted"] != true {
		t.Fatalf("post result=%v err=%v", result, err)
	}
	if _, err := client.PostBytes(
		context.Background(), "post", "/post", url.Values{"q": []string{"value"}},
		map[string]string{"input": "value"},
	); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(context.Background()); err != nil || logoutCalls.Load() != 1 {
		t.Fatalf("logout calls=%d err=%v", logoutCalls.Load(), err)
	}
	if observer.count("success") == 0 || observer.count("error") == 0 {
		t.Fatalf("observer records = %#v", observer.records)
	}
	if observer.responseSizeCount() == 0 {
		t.Fatal("API response sizes were not observed")
	}
}

func TestClientSafeFailures(t *testing.T) {
	t.Parallel()
	var codedServerErrors atomic.Int32
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			writer.Header().Set(tokenHeader, "token")
			writer.WriteHeader(http.StatusOK)
		case "/invalid":
			_, _ = writer.Write([]byte(`not-json`))
		case "/missing":
			http.Error(writer, "sensitive response body", http.StatusNotFound)
		case "/coded":
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"code":"999","details":"sensitive response body"}`))
		case "/coded-server-error":
			codedServerErrors.Add(1)
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte(`{"code":999,"details":"sensitive response body"}`))
		case "/redirect":
			http.Redirect(writer, request, "https://example.invalid/private", http.StatusFound)
		default:
			http.NotFound(writer, request)
		}
	})
	client := newTestClient(t, handler, nil)

	for _, test := range []struct {
		name string
		run  func() error
		want string
	}{
		{"unsafe path", func() error {
			return client.GetJSON(context.Background(), "x", "//other", nil, nil)
		}, "safe relative path"},
		{"missing logical", func() error {
			return client.GetJSON(context.Background(), "", "/data", nil, nil)
		}, "logical API"},
		{"invalid body", func() error {
			return client.PostJSON(context.Background(), "post", "/post", nil, math.NaN(), nil)
		}, "encode request"},
		{"invalid response", func() error {
			return client.GetJSON(context.Background(), "invalid", "/invalid", nil, &map[string]any{})
		}, "invalid_response"},
		{"http response", func() error {
			return client.GetJSON(context.Background(), "missing", "/missing", nil, nil)
		}, "status 404"},
		{"redirect", func() error {
			return client.GetJSON(context.Background(), "redirect", "/redirect", nil, nil)
		}, "transport"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			err := test.run()
			if err == nil || !strings.Contains(err.Error(), test.want) ||
				strings.Contains(err.Error(), "sensitive response body") ||
				strings.Contains(err.Error(), "example.invalid/private") {
				t.Fatalf("error = %v", err)
			}
		})
	}

	_, err := client.GetBytes(context.Background(), "coded", "/coded", nil)
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Status != http.StatusBadRequest ||
		apiError.Code != 999 || strings.Contains(err.Error(), "sensitive response body") {
		t.Fatalf("coded API error = %#v err=%v", apiError, err)
	}
	_, err = client.PostBytes(
		context.Background(), "bucket-billing-batch", "/coded-server-error", nil, nil,
	)
	apiError = nil
	if !errors.As(err, &apiError) || apiError.Status != http.StatusInternalServerError ||
		apiError.Code != 999 || codedServerErrors.Load() != 3 ||
		strings.Contains(err.Error(), "sensitive response body") {
		t.Fatalf(
			"coded server API error = %#v calls=%d err=%v",
			apiError, codedServerErrors.Load(), err,
		)
	}
}

func TestClientRetriesLoginFailure(t *testing.T) {
	t.Parallel()
	var logins atomic.Int32
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			if logins.Add(1) == 1 {
				http.Error(writer, "temporary", http.StatusServiceUnavailable)
				return
			}
			writer.Header().Set(tokenHeader, "token")
		case "/data":
			_, _ = writer.Write([]byte(`{"ok":true}`))
		}
	})
	client := newTestClient(t, handler, nil)
	var response map[string]any
	if err := client.GetJSON(context.Background(), "data", "/data", nil, &response); err != nil ||
		response["ok"] != true || logins.Load() != 2 {
		t.Fatalf("response=%v logins=%d err=%v", response, logins.Load(), err)
	}
}

func TestAuthManagerMissingTokenCookieAndLogoutErrors(t *testing.T) {
	t.Parallel()
	var mode atomic.Int32
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			switch mode.Load() {
			case 0:
				writer.WriteHeader(http.StatusOK)
			case 1:
				http.SetCookie(writer, &http.Cookie{Name: tokenHeader, Value: "cookie-token"})
			default:
				http.Error(writer, "no", http.StatusForbidden)
			}
		case "/logout":
			http.Error(writer, "no", http.StatusInternalServerError)
		}
	})
	base, _ := url.Parse("https://ecs.example.invalid")
	httpClient := &http.Client{Transport: handlerTransport{handler: handler}}
	manager := NewAuthManager("alpha", base, httpClient, "u", "p", NopObserver{})
	if _, err := manager.Token(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "missing_token") {
		t.Fatalf("missing token error = %v", err)
	}
	mode.Store(1)
	token, err := manager.Token(context.Background(), 1)
	if err != nil || token != "cookie-token" {
		t.Fatalf("cookie token = %q err=%v", token, err)
	}
	if err := manager.Logout(context.Background()); err == nil {
		t.Fatal("logout status error was not returned")
	}
	mode.Store(2)
	manager.Invalidate()
	if _, err := manager.Token(context.Background(), 1); err == nil {
		t.Fatal("login status error was not returned")
	}
}

func TestClientHelpersAndCircuitBreaker(t *testing.T) {
	t.Parallel()
	if _, err := managementURL("http://ecs.example.invalid"); err == nil {
		t.Fatal("HTTP endpoint was accepted")
	}
	for _, endpoint := range []string{
		"https://user@ecs.example.invalid",
		"https://ecs.example.invalid/path",
		"https://ecs.example.invalid?token=secret",
	} {
		if _, err := managementURL(endpoint); err == nil {
			t.Fatalf("unsafe endpoint %q was accepted", endpoint)
		}
	}
	parsed, err := managementURL("https://ecs.example.invalid")
	if err != nil || parsed.Port() != "4443" {
		t.Fatalf("management URL = %v err=%v", parsed, err)
	}
	if !validRelativePath("/vdc/nodes") || validRelativePath("//evil") ||
		validRelativePath("/path?secret=x") {
		t.Fatal("relative path validation failed")
	}
	if _, err := readBounded(strings.NewReader("12345"), 4); err == nil {
		t.Fatal("oversized response accepted")
	}
	if data, err := readBounded(strings.NewReader("1234"), 4); err != nil || string(data) != "1234" {
		t.Fatalf("bounded response = %q err=%v", data, err)
	}
	if isRetryableTransport(context.Canceled) || !isRetryableTransport(context.DeadlineExceeded) ||
		!isRetryableTransport(&net.DNSError{IsTimeout: true}) {
		t.Fatal("transport classification is incorrect")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	retry := config.Defaults().Collector.Retry
	if err := waitBackoff(ctx, retry, 1, 0); err == nil {
		t.Fatal("canceled backoff returned nil")
	}
	retry.InitialBackoff.Duration = time.Duration(math.MaxInt64 / 2)
	retry.MaxBackoff.Duration = time.Duration(math.MaxInt64)
	if err := waitBackoff(ctx, retry, 10, 0); err == nil {
		t.Fatal("canceled maximum backoff returned nil")
	}
	limiter := newRequestLimiter(1, 1)
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := limiter.Wait(ctx); err == nil {
		t.Fatal("rate limiter ignored canceled context")
	}
	now := time.Now()
	if parseRetryAfter("2", now) != 2*time.Second ||
		parseRetryAfter(now.Add(time.Second).UTC().Format(http.TimeFormat), now) <= 0 ||
		parseRetryAfter("9223372036854775807", now) != 0 ||
		parseRetryAfter("invalid", now) != 0 {
		t.Fatal("Retry-After parsing failed")
	}
	for _, test := range []struct {
		body string
		code int
	}{
		{`{"code":999}`, 999},
		{`{"code":"999"}`, 999},
		{`{"code":-1}`, 0},
		{`{"code":"not-a-number"}`, 0},
		{`{"message":"missing"}`, 0},
		{`not-json`, 0},
	} {
		if code := parseAPIErrorCode([]byte(test.body)); code != test.code {
			t.Fatalf("parse API error code %q = %d, want %d", test.body, code, test.code)
		}
	}

	var breaker circuitBreaker
	for range 5 {
		breaker.Failure(now)
	}
	if err := breaker.Allow(now); err == nil {
		t.Fatal("open circuit allowed a request")
	}
	if err := breaker.Allow(now.Add(31 * time.Second)); err != nil {
		t.Fatal(err)
	}
	breaker.Failure(now)
	breaker.Success()
	if err := breaker.Allow(now); err != nil {
		t.Fatal(err)
	}
}

func TestClientConfigurationAndCAValidation(t *testing.T) {
	t.Parallel()
	cluster := testClientCluster("https://ecs.example.invalid")
	settings := config.Defaults().Collector
	settings.DefaultTimeout.Duration = 0
	if _, err := NewClient(cluster, settings, nil); err == nil {
		t.Fatal("invalid client settings accepted")
	}
	settings = config.Defaults().Collector
	cluster.TLS.CAFile = filepath.Join(t.TempDir(), "missing.pem")
	if _, err := NewClient(cluster, settings, nil); err == nil {
		t.Fatal("missing CA accepted")
	}
	caPath := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(caPath, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	cluster.TLS.CAFile = caPath
	if _, err := NewClient(cluster, settings, nil); err == nil {
		t.Fatal("invalid CA accepted")
	}
}

type recordingObserver struct {
	mu            sync.Mutex
	records       []string
	responseSizes []int64
}

func (o *recordingObserver) ObserveAPIResponseSize(
	_, _, _ string,
	bytes int64,
) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.responseSizes = append(o.responseSizes, bytes)
}

func (o *recordingObserver) responseSizeCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.responseSizes)
}

func (o *recordingObserver) ObserveAPI(
	_, _, result, _, _ string, _, _ int, _ time.Duration,
) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.records = append(o.records, result)
}

func (o *recordingObserver) count(result string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	count := 0
	for _, value := range o.records {
		if value == result {
			count++
		}
	}
	return count
}

func newTestClient(t *testing.T, handler http.Handler, observer Observer) *Client {
	t.Helper()
	client, err := NewClient(
		testClientCluster("https://ecs.example.invalid:4443"), testClientSettings(), observer,
	)
	if err != nil {
		t.Fatal(err)
	}
	client.http.Transport = handlerTransport{handler: handler}
	return client
}

func testClientCluster(endpoint string) config.ClusterConfig {
	verify := false
	return config.ClusterConfig{
		Name: "alpha", Environment: "test", Endpoint: endpoint,
		Username: "monitor", Password: "secret", TLS: config.ClusterTLSConfig{Verify: &verify},
	}
}

func testClientSettings() config.CollectorConfig {
	value := config.Defaults().Collector
	value.DefaultTimeout.Duration = 2 * time.Second
	value.Retry.MaxAttempts = 3
	value.Retry.InitialBackoff.Duration = time.Millisecond
	value.Retry.MaxBackoff.Duration = 2 * time.Millisecond
	value.RateLimit.RequestsPerSecond = 10_000
	value.RateLimit.Burst = 100
	return value
}

type handlerTransport struct {
	handler http.Handler
}

func (t handlerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	response.Request = request
	return response, nil
}
