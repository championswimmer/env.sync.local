package httptransport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"envsync/internal/config"
	"envsync/internal/secrets"
)

var hostPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
	Host      string `json:"host"`
}

func FetchHealth(dial string) (HealthResponse, error) {
	host, port := secrets.SplitDial(dial)
	if err := validateHost(host); err != nil {
		return HealthResponse{}, err
	}
	if port == "" {
		port = config.EnvSyncPort()
	}
	url := fmt.Sprintf("http://%s:%s/health", host, port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return HealthResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return HealthResponse{}, err
	}
	var health HealthResponse
	if err := json.Unmarshal(body, &health); err != nil {
		return HealthResponse{}, err
	}
	if health.Status == "" {
		return HealthResponse{}, errors.New("missing status")
	}
	return health, nil
}

func FetchSecrets(dial string) ([]byte, error) {
	host, port := secrets.SplitDial(dial)
	if err := validateHost(host); err != nil {
		return nil, err
	}
	if port == "" {
		port = config.EnvSyncPort()
	}
	url := fmt.Sprintf("http://%s:%s/secrets.env", host, port)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func validateHost(host string) error {
	if host == "" || !hostPattern.MatchString(host) {
		return errors.New("invalid host")
	}
	return nil
}
