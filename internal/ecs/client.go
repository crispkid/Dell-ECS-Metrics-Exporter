package ecs

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dell-ecs-metrics-exporter/internal/config"
)

const (
	tokenHeader     = "X-SDS-AUTH-TOKEN"
	maxResponseSize = 32 << 20
)

type Observer interface {
	ObserveAPI(
		cluster, logical, result, errorType, correlationID string,
		status, attempt int,
		duration time.Duration,
	)
}

type ResponseSizeObserver interface {
	ObserveAPIResponseSize(cluster, logical, result string, bytes int64)
}

type NopObserver struct{}

func (NopObserver) ObserveAPI(string, string, string, string, string, int, int, time.Duration) {}

type correlationKey struct{}

var correlationSequence atomic.Uint64

func WithCorrelationID(ctx context.Context, cluster string) context.Context {
	if value, ok := ctx.Value(correlationKey{}).(string); ok && value != "" {
		return ctx
	}
	value := fmt.Sprintf("%s-%d", labelFragment(cluster), correlationSequence.Add(1))
	return context.WithValue(ctx, correlationKey{}, value)
}

func CorrelationID(ctx context.Context) string {
	if value, ok := ctx.Value(correlationKey{}).(string); ok {
		return value
	}
	return ""
}

type APIError struct {
	Logical   string
	Status    int
	Code      int
	Kind      string
	Retryable bool
}

func (e *APIError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("ECS logical API %s failed with status %d (%s)", e.Logical, e.Status, e.Kind)
	}
	return fmt.Sprintf("ECS logical API %s failed (%s)", e.Logical, e.Kind)
}

type Client struct {
	cluster   string
	baseURL   *url.URL
	http      *http.Client
	auth      *AuthManager
	observer  Observer
	semaphore chan struct{}
	limiter   *requestLimiter
	timeout   time.Duration
	retry     config.RetryConfig
	breaker   circuitBreaker
}

func NewClient(cluster config.ClusterConfig, settings config.CollectorConfig, observer Observer) (*Client, error) {
	if settings.DefaultTimeout.Duration <= 0 ||
		settings.Concurrency.MaxConcurrentRequestsPerCluster < 1 ||
		settings.RateLimit.RequestsPerSecond <= 0 ||
		settings.RateLimit.Burst < 1 ||
		settings.Retry.MaxAttempts < 1 ||
		settings.Retry.InitialBackoff.Duration <= 0 ||
		settings.Retry.MaxBackoff.Duration < settings.Retry.InitialBackoff.Duration {
		return nil, fmt.Errorf("invalid ECS client timeout, concurrency, or retry settings")
	}
	username, password, err := cluster.Credentials()
	if err != nil {
		return nil, err
	}
	baseURL, err := managementURL(cluster.Endpoint)
	if err != nil {
		return nil, err
	}
	connectTimeout, readTimeout, overallTimeout :=
		cluster.Timeouts.Resolve(settings.DefaultTimeout.Duration)
	if connectTimeout <= 0 || readTimeout <= 0 || overallTimeout <= 0 ||
		connectTimeout > overallTimeout || readTimeout > overallTimeout {
		return nil, fmt.Errorf("invalid ECS client timeout overrides")
	}
	transport, err := newTransport(cluster.TLS, connectTimeout, readTimeout)
	if err != nil {
		return nil, fmt.Errorf("cluster %q TLS: %w", cluster.Name, err)
	}
	httpClient := &http.Client{
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			origin := via[0].URL
			if request.URL.Scheme != origin.Scheme || request.URL.Host != origin.Host {
				return fmt.Errorf("cross-origin redirect rejected")
			}
			return nil
		},
	}
	if observer == nil {
		observer = NopObserver{}
	}
	client := &Client{
		cluster: cluster.Name, baseURL: baseURL, http: httpClient, observer: observer,
		semaphore: make(chan struct{}, settings.Concurrency.MaxConcurrentRequestsPerCluster),
		limiter: newRequestLimiter(
			settings.RateLimit.RequestsPerSecond, settings.RateLimit.Burst,
		),
		timeout: overallTimeout, retry: settings.Retry,
	}
	client.auth = newAuthManagerWithLimiter(
		cluster.Name, baseURL, httpClient, username, password, observer, client.limiter,
	)
	return client, nil
}

func (c *Client) GetJSON(ctx context.Context, logical, path string, query url.Values, target any) error {
	return c.DoJSON(ctx, logical, http.MethodGet, path, query, nil, target)
}

func (c *Client) PostJSON(ctx context.Context, logical, path string, query url.Values, body, target any) error {
	return c.DoJSON(ctx, logical, http.MethodPost, path, query, body, target)
}

func (c *Client) GetBytes(ctx context.Context, logical, path string, query url.Values) ([]byte, error) {
	var raw json.RawMessage
	if err := c.DoJSON(ctx, logical, http.MethodGet, path, query, nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) PostBytes(ctx context.Context, logical, path string, query url.Values, body any) ([]byte, error) {
	var raw json.RawMessage
	if err := c.DoJSON(ctx, logical, http.MethodPost, path, query, body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) DoJSON(
	ctx context.Context,
	logical, method, path string,
	query url.Values,
	body any,
	target any,
) error {
	if logical == "" || !validRelativePath(path) {
		return fmt.Errorf("logical API and safe relative path are required")
	}
	if err := c.breaker.Allow(time.Now()); err != nil {
		return &APIError{Logical: logical, Kind: "circuit_open", Retryable: true}
	}
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return &APIError{Logical: logical, Kind: "context", Retryable: false}
	}

	bodyData, err := marshalRequestBody(body)
	if err != nil {
		return fmt.Errorf("encode request for %s: %w", logical, err)
	}
	requestContext, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	requestContext = WithCorrelationID(requestContext, c.cluster)

	var last error
	reauthenticated := false
	httpAttempt := 0
	for attempt := 1; attempt <= c.retry.MaxAttempts; attempt++ {
		token, err := c.auth.Token(requestContext, max(httpAttempt+1, attempt))
		if err != nil {
			last = err
			apiError := new(APIError)
			if !errors.As(err, &apiError) || !apiError.Retryable ||
				attempt == c.retry.MaxAttempts {
				break
			}
			if err := waitBackoff(requestContext, c.retry, attempt, 0); err != nil {
				last = &APIError{Logical: logical, Kind: "context"}
				break
			}
			continue
		}
		httpAttempt++
		responseData, status, duration, retryAfter, err := c.request(
			requestContext, logical, method, path, query, bodyData, token, httpAttempt,
		)
		if status == http.StatusUnauthorized && !reauthenticated {
			c.auth.Invalidate()
			reauthenticated = true
			attempt--
			continue
		}
		if err == nil {
			if target != nil && len(responseData) != 0 {
				if decodeErr := json.Unmarshal(responseData, target); decodeErr != nil {
					err = &APIError{Logical: logical, Status: status, Kind: "invalid_response"}
					c.observer.ObserveAPI(
						c.cluster, logical, "error", "invalid_response",
						CorrelationID(requestContext), status, httpAttempt, duration,
					)
				} else {
					c.observer.ObserveAPI(
						c.cluster, logical, "success", "none",
						CorrelationID(requestContext), status, httpAttempt, duration,
					)
					c.breaker.Success()
					return nil
				}
			} else {
				c.observer.ObserveAPI(
					c.cluster, logical, "success", "none",
					CorrelationID(requestContext), status, httpAttempt, duration,
				)
				c.breaker.Success()
				return nil
			}
		}
		last = err
		apiError := new(APIError)
		if !errors.As(err, &apiError) || !apiError.Retryable || attempt == c.retry.MaxAttempts {
			break
		}
		if err := waitBackoff(requestContext, c.retry, attempt, retryAfter); err != nil {
			last = &APIError{Logical: logical, Kind: "context"}
			break
		}
	}
	if apiError := new(APIError); errors.As(last, &apiError) && apiError.Retryable {
		c.breaker.Failure(time.Now())
	}
	return last
}

func (c *Client) request(
	ctx context.Context,
	logical, method, path string,
	query url.Values,
	body []byte,
	token string,
	attempt int,
) ([]byte, int, time.Duration, time.Duration, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, 0, 0, 0, &APIError{Logical: logical, Kind: "rate_limit_context"}
	}
	endpoint := *c.baseURL
	decodedPath, err := url.PathUnescape(path)
	if err != nil {
		return nil, 0, 0, 0, &APIError{Logical: logical, Kind: "request"}
	}
	endpoint.Path = decodedPath
	endpoint.RawPath = path
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, 0, 0, 0, &APIError{Logical: logical, Kind: "request"}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set(tokenHeader, token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	started := time.Now()
	response, err := c.http.Do(request)
	duration := time.Since(started)
	correlationID := CorrelationID(ctx)
	if err != nil {
		retryable := isRetryableTransport(err)
		c.observer.ObserveAPI(
			c.cluster, logical, "error", "transport", correlationID, 0, attempt, duration,
		)
		return nil, 0, duration, 0, &APIError{Logical: logical, Kind: "transport", Retryable: retryable}
	}
	defer response.Body.Close()
	data, readErr := readBounded(response.Body, maxResponseSize)
	duration = time.Since(started)
	if readErr != nil {
		observeResponseSize(c.observer, c.cluster, logical, "error", maxResponseSize+1)
		c.observer.ObserveAPI(
			c.cluster, logical, "error", "response_size", correlationID,
			response.StatusCode, attempt, duration,
		)
		return nil, response.StatusCode, duration, 0, &APIError{
			Logical: logical, Status: response.StatusCode, Kind: "response_size",
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		observeResponseSize(c.observer, c.cluster, logical, "error", int64(len(data)))
		retryable := retryableStatus(response.StatusCode)
		kind := "http_" + strconv.Itoa(response.StatusCode)
		if response.StatusCode == http.StatusUnauthorized {
			kind = "authentication"
		}
		c.observer.ObserveAPI(
			c.cluster, logical, "error", kind, correlationID,
			response.StatusCode, attempt, duration,
		)
		return data, response.StatusCode, duration,
			parseRetryAfter(response.Header.Get("Retry-After"), time.Now()), &APIError{
				Logical: logical, Status: response.StatusCode, Code: parseAPIErrorCode(data),
				Kind: kind, Retryable: retryable,
			}
	}
	observeResponseSize(c.observer, c.cluster, logical, "success", int64(len(data)))
	return data, response.StatusCode, duration, 0, nil
}

func parseAPIErrorCode(data []byte) int {
	var envelope struct {
		Code json.RawMessage `json:"code"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Code) == 0 {
		return 0
	}
	var number json.Number
	if err := json.Unmarshal(envelope.Code, &number); err == nil {
		value, err := strconv.Atoi(number.String())
		if err == nil && value > 0 {
			return value
		}
	}
	var text string
	if err := json.Unmarshal(envelope.Code, &text); err == nil {
		value, err := strconv.Atoi(text)
		if err == nil && value > 0 {
			return value
		}
	}
	return 0
}

func (c *Client) Close(ctx context.Context) error {
	return c.auth.Logout(ctx)
}

type AuthManager struct {
	cluster  string
	baseURL  *url.URL
	http     *http.Client
	username string
	password string
	observer Observer
	limiter  *requestLimiter
	mu       sync.Mutex
	token    string
}

func NewAuthManager(
	cluster string,
	baseURL *url.URL,
	httpClient *http.Client,
	username, password string,
	observer Observer,
) *AuthManager {
	return newAuthManagerWithLimiter(
		cluster, baseURL, httpClient, username, password, observer, nil,
	)
}

func newAuthManagerWithLimiter(
	cluster string,
	baseURL *url.URL,
	httpClient *http.Client,
	username, password string,
	observer Observer,
	limiter *requestLimiter,
) *AuthManager {
	if observer == nil {
		observer = NopObserver{}
	}
	return &AuthManager{
		cluster: cluster, baseURL: baseURL, http: httpClient,
		username: username, password: password, observer: observer, limiter: limiter,
	}
}

func (a *AuthManager) Token(ctx context.Context, attempt int) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" {
		return a.token, nil
	}
	if a.limiter != nil {
		if err := a.limiter.Wait(ctx); err != nil {
			return "", &APIError{Logical: "login", Kind: "rate_limit_context"}
		}
	}
	endpoint := *a.baseURL
	endpoint.Path = "/login"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", &APIError{Logical: "login", Kind: "request"}
	}
	request.SetBasicAuth(a.username, a.password)
	request.Header.Set("Accept", "application/json")
	started := time.Now()
	response, err := a.http.Do(request)
	duration := time.Since(started)
	correlationID := CorrelationID(ctx)
	if err != nil {
		a.observer.ObserveAPI(a.cluster, "login", "error", "transport", correlationID, 0, attempt, duration)
		return "", &APIError{Logical: "login", Kind: "transport", Retryable: isRetryableTransport(err)}
	}
	defer response.Body.Close()
	responseBytes, _ := io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	result := "success"
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		result = "error"
	}
	observeResponseSize(a.observer, a.cluster, "login", result, responseBytes)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		kind := "http_" + strconv.Itoa(response.StatusCode)
		if response.StatusCode == http.StatusUnauthorized ||
			response.StatusCode == http.StatusForbidden {
			kind = "authentication"
		}
		a.observer.ObserveAPI(
			a.cluster, "login", "error", kind, correlationID,
			response.StatusCode, attempt, duration,
		)
		return "", &APIError{
			Logical: "login", Status: response.StatusCode, Kind: kind,
			Retryable: retryableStatus(response.StatusCode),
		}
	}
	token := response.Header.Get(tokenHeader)
	if token == "" {
		for _, cookie := range response.Cookies() {
			if strings.EqualFold(cookie.Name, tokenHeader) {
				token = cookie.Value
				break
			}
		}
	}
	if len(token) > 4096 || strings.ContainsAny(token, "\r\n") {
		a.observer.ObserveAPI(
			a.cluster, "login", "error", "invalid_token", correlationID,
			response.StatusCode, attempt, duration,
		)
		return "", &APIError{Logical: "login", Status: response.StatusCode, Kind: "invalid_token"}
	}
	if token == "" {
		a.observer.ObserveAPI(
			a.cluster, "login", "error", "missing_token", correlationID,
			response.StatusCode, attempt, duration,
		)
		return "", &APIError{Logical: "login", Status: response.StatusCode, Kind: "missing_token"}
	}
	a.token = token
	a.observer.ObserveAPI(
		a.cluster, "login", "success", "none", correlationID,
		response.StatusCode, attempt, duration,
	)
	return a.token, nil
}

func (a *AuthManager) Invalidate() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = ""
}

func (a *AuthManager) Logout(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token == "" {
		return nil
	}
	ctx = WithCorrelationID(ctx, a.cluster)
	if a.limiter != nil {
		if err := a.limiter.Wait(ctx); err != nil {
			return &APIError{Logical: "logout", Kind: "rate_limit_context"}
		}
	}
	endpoint := *a.baseURL
	endpoint.Path = "/logout"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return &APIError{Logical: "logout", Kind: "request"}
	}
	request.Header.Set(tokenHeader, a.token)
	started := time.Now()
	response, err := a.http.Do(request)
	duration := time.Since(started)
	a.token = ""
	if err != nil {
		a.observer.ObserveAPI(
			a.cluster, "logout", "error", "transport", CorrelationID(ctx), 0, 1, duration,
		)
		return &APIError{Logical: "logout", Kind: "transport", Retryable: isRetryableTransport(err)}
	}
	defer response.Body.Close()
	responseBytes, _ := io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	result := "success"
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		result = "error"
	}
	observeResponseSize(a.observer, a.cluster, "logout", result, responseBytes)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		a.observer.ObserveAPI(
			a.cluster, "logout", "error", "http", CorrelationID(ctx),
			response.StatusCode, 1, duration,
		)
		return &APIError{Logical: "logout", Status: response.StatusCode, Kind: "http"}
	}
	a.observer.ObserveAPI(
		a.cluster, "logout", "success", "none", CorrelationID(ctx),
		response.StatusCode, 1, duration,
	)
	return nil
}

type circuitBreaker struct {
	mu        sync.Mutex
	failures  int
	openUntil time.Time
}

func (b *circuitBreaker) Allow(now time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if now.Before(b.openUntil) {
		return fmt.Errorf("circuit is open")
	}
	if !b.openUntil.IsZero() {
		b.openUntil = time.Time{}
		b.failures = 0
	}
	return nil
}

func (b *circuitBreaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openUntil = time.Time{}
}

func (b *circuitBreaker) Failure(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= 5 {
		b.openUntil = now.Add(30 * time.Second)
	}
}

func newTransport(
	settings config.ClusterTLSConfig,
	connectTimeout, readTimeout time.Duration,
) (*http.Transport, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// Verification is enabled by default. Operators may explicitly disable
		// it for ECS deployments whose self-signed certificate cannot be
		// represented by caFile; startup emits a warning for that choice.
		InsecureSkipVerify: !settings.VerificationEnabled(), // #nosec G402
	}
	if settings.CAFile != "" {
		content, err := os.ReadFile(settings.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(content) {
			return nil, fmt.Errorf("CA file contains no certificates")
		}
		tlsConfig.RootCAs = roots
	}
	dialer := &net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment, DialContext: dialer.DialContext,
		TLSClientConfig: tlsConfig, ResponseHeaderTimeout: readTimeout,
		TLSHandshakeTimeout: min(connectTimeout, 10*time.Second),
		IdleConnTimeout:     90 * time.Second, MaxIdleConns: 100, MaxIdleConnsPerHost: 10,
		MaxResponseHeaderBytes: 1 << 20, ForceAttemptHTTP2: true,
	}, nil
}

func managementURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("invalid HTTPS endpoint")
	}
	if parsed.Port() == "" {
		parsed.Host = net.JoinHostPort(parsed.Hostname(), "4443")
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

func validRelativePath(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") &&
		!strings.ContainsAny(value, "?#")
}

func marshalRequestBody(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds limit")
	}
	return data, nil
}

type requestLimiter struct {
	mu                sync.Mutex
	requestsPerSecond float64
	burst             float64
	tokens            float64
	last              time.Time
}

func newRequestLimiter(requestsPerSecond float64, burst int) *requestLimiter {
	return &requestLimiter{
		requestsPerSecond: requestsPerSecond,
		burst:             float64(burst), tokens: float64(burst), last: time.Now(),
	}
}

func (l *requestLimiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(l.last).Seconds()
		if elapsed > 0 {
			l.tokens = min(l.burst, l.tokens+elapsed*l.requestsPerSecond)
			l.last = now
		}
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		wait := time.Duration((1 - l.tokens) / l.requestsPerSecond * float64(time.Second))
		l.mu.Unlock()
		if wait <= 0 {
			wait = time.Nanosecond
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		}
	}
}

func observeResponseSize(
	observer Observer,
	cluster, logical, result string,
	bytes int64,
) {
	if sized, ok := observer.(ResponseSizeObserver); ok {
		sized.ObserveAPIResponseSize(cluster, logical, result, bytes)
	}
}

func isRetryableTransport(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError)
}

func waitBackoff(
	ctx context.Context,
	retry config.RetryConfig,
	attempt int,
	retryAfter time.Duration,
) error {
	delay := retry.InitialBackoff.Duration
	for current := 1; current < attempt && delay < retry.MaxBackoff.Duration; current++ {
		if delay > retry.MaxBackoff.Duration/2 {
			delay = retry.MaxBackoff.Duration
			break
		}
		delay *= 2
	}
	if retryAfter > delay {
		delay = min(retryAfter, retry.MaxBackoff.Duration)
	}
	jitterLimit := min(delay/5, time.Duration(math.MaxInt64-int64(delay)))
	var jitter time.Duration
	if jitterLimit > 0 {
		jitter = time.Duration(rand.Int64N(int64(jitterLimit)))
	}
	timer := time.NewTimer(delay + jitter)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		if seconds <= 0 || int64(seconds) > math.MaxInt64/int64(time.Second) {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if timestamp, err := http.ParseTime(value); err == nil && timestamp.After(now) {
		return timestamp.Sub(now)
	}
	return 0
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusInternalServerError ||
		status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

func labelFragment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ecs"
	}
	var result strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' {
			result.WriteRune(character)
		}
		if result.Len() >= 32 {
			break
		}
	}
	if result.Len() == 0 {
		return "ecs"
	}
	return result.String()
}
