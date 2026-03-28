package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// -----------------------------------------------------------------------------
// Configuration
// -----------------------------------------------------------------------------

type config struct {
	Port           string
	MetricsPath    string
	ScrapeInterval time.Duration
	Username       string
	Password       string
	LoginURL       string
	UsersURL       string
}

func loadConfig() config {
	base := "https://api." + getEnvOrDefault("TORR9_API_BASE_URL", "torr9.net")
	return config{
		Port:           getEnvOrDefault("PORT", "9090"),
		MetricsPath:    getEnvOrDefault("METRICS_PATH", "/metrics"),
		Username:       os.Getenv("TORR9_USERNAME"),
		Password:       os.Getenv("TORR9_PASSWORD"),
		ScrapeInterval: parseDuration(os.Getenv("SCRAPE_INTERVAL"), 5*time.Minute),
		LoginURL:       base + "/api/v1/auth/login",
		UsersURL:       base + "/api/v1/users/me",
	}
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// -----------------------------------------------------------------------------
// API client
// -----------------------------------------------------------------------------

// Torr9Client manages authentication and communication with the Torr9 API.
type Torr9Client struct {
	mu         sync.RWMutex
	token      string
	loginURL   string
	usersURL   string
	httpClient *http.Client
}

func newTorr9Client(loginURL, usersURL string) *Torr9Client {
	return &Torr9Client{
		loginURL:   loginURL,
		usersURL:   usersURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Torr9Client) IsAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token != ""
}

// Login authenticates with the Torr9 API and stores the returned token.
func (c *Torr9Client) Login(username, password string) error {
	payload, err := json.Marshal(map[string]interface{}{
		"username":    username,
		"password":    password,
		"remember_me": true,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal login payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.loginURL, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("failed to build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid credentials")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected login response: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	var loginResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}
	if loginResp.Token == "" {
		return fmt.Errorf("login response contained no token")
	}

	c.mu.Lock()
	c.token = loginResp.Token
	c.mu.Unlock()

	fmt.Printf("[auth] Successfully authenticated as %s\n", username)
	return nil
}

// FetchMetrics retrieves the current user's metrics from the Torr9 API.
func (c *Torr9Client) FetchMetrics() (*UserMetrics, error) {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	req, err := http.NewRequest(http.MethodGet, c.usersURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build metrics request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("metrics request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("authentication required")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected metrics response: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics response: %w", err)
	}

	var metrics UserMetrics
	if err := json.Unmarshal(body, &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse metrics response: %w", err)
	}
	return &metrics, nil
}

// -----------------------------------------------------------------------------
// Domain types
// -----------------------------------------------------------------------------

type UserMetrics struct {
	TotalUploadedBytes   int64  `json:"total_uploaded_bytes"`
	TotalDownloadedBytes int64  `json:"total_downloaded_bytes"`
	JetonBalance         int64  `json:"jeton_balance"`
	Username             string `json:"username"`
}

// -----------------------------------------------------------------------------
// Prometheus metrics
// -----------------------------------------------------------------------------

type exporterMetrics struct {
	totalUploaded   prometheus.Gauge
	totalDownloaded prometheus.Gauge
	jetonBalance    prometheus.Gauge
}

func newExporterMetrics(reg prometheus.Registerer) *exporterMetrics {
	m := &exporterMetrics{
		totalUploaded: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "torr9",
			Name:      "total_uploaded_bytes",
			Help:      "Total uploaded bytes from Torr9 API",
		}),
		totalDownloaded: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "torr9",
			Name:      "total_downloaded_bytes",
			Help:      "Total downloaded bytes from Torr9 API",
		}),
		jetonBalance: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "torr9",
			Name:      "jeton_balance",
			Help:      "Jeton balance from Torr9 API",
		}),
	}
	reg.MustRegister(m.totalUploaded, m.totalDownloaded, m.jetonBalance)
	return m
}

func (m *exporterMetrics) update(u *UserMetrics) {
	m.totalUploaded.Set(float64(u.TotalUploadedBytes))
	m.totalDownloaded.Set(float64(u.TotalDownloadedBytes))
	m.jetonBalance.Set(float64(u.JetonBalance))
}

// -----------------------------------------------------------------------------
// HTTP handlers
// -----------------------------------------------------------------------------

type server struct {
	client  *Torr9Client
	metrics *exporterMetrics
}

func (s *server) metricsHandler(c *gin.Context) {
	if s.client.IsAuthenticated() {
		userMetrics, err := s.client.FetchMetrics()
		if err != nil {
			fmt.Printf("[metrics] Error scraping metrics: %v\n", err)
		} else {
			s.metrics.update(userMetrics)
		}
	}

	c.Header("Content-Type", "text/plain; version=0.0.4")
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}

func (s *server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":        "healthy",
		"authenticated": s.client.IsAuthenticated(),
	})
}

// -----------------------------------------------------------------------------
// Entrypoint
// -----------------------------------------------------------------------------

func main() {
	cfg := loadConfig()

	client := newTorr9Client(cfg.LoginURL, cfg.UsersURL)
	metrics := newExporterMetrics(prometheus.DefaultRegisterer)
	srv := &server{client: client, metrics: metrics}

	// Attempt auto-login on startup if credentials are provided.
	if cfg.Username != "" && cfg.Password != "" {
		fmt.Println("[auth] Credentials found, attempting auto-login...")
		if err := client.Login(cfg.Username, cfg.Password); err != nil {
			fmt.Printf("[auth] Auto-login failed (non-fatal): %v\n", err)
		}
	} else {
		fmt.Println("[auth] No credentials provided, auto-login skipped")
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET(cfg.MetricsPath, srv.metricsHandler)
	r.GET("/health", srv.healthHandler)

	addr := ":" + cfg.Port
	fmt.Printf("[server] Starting Torr9 exporter on %s\n", addr)
	fmt.Printf("[server] Metrics available at: http://localhost%s%s\n", addr, cfg.MetricsPath)
	fmt.Printf("[server] Scrape interval: %v\n", cfg.ScrapeInterval)

	if err := r.Run(addr); err != nil {
		fmt.Printf("[server] Error starting server: %v\n", err)
		os.Exit(1)
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// parseDuration parses a simple duration string (e.g. "30s", "5m", "2h").
// Returns fallback if the string is empty or cannot be parsed.
func parseDuration(s string, fallback time.Duration) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	units := []struct {
		suffix string
		mult   time.Duration
	}{
		{"h", time.Hour},
		{"m", time.Minute},
		{"s", time.Second},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			numStr := strings.TrimSuffix(s, u.suffix)
			var n int
			if _, err := fmt.Sscanf(numStr, "%d", &n); err == nil && n > 0 {
				return time.Duration(n) * u.mult
			}
		}
	}
	return fallback
}
