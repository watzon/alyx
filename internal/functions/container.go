package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	// containerLabelKey is the label used to identify Alyx containers.
	containerLabelKey = "io.alyx.managed"
	// containerRuntimeLabel is the label for the runtime type.
	containerRuntimeLabel = "io.alyx.runtime"
	// executorPort is the port the executor listens on inside the container.
	executorPort = 8080
	// healthCheckPath is the path to check container health.
	healthCheckPath = "/health"
	// invokePath is the path to invoke functions.
	invokePath = "/invoke"
	// containerReadyTimeoutSeconds is the timeout for waiting for a container to be ready.
	containerReadyTimeoutSeconds = 30
	// containerIDLogLength is the length of container ID to show in logs.
	containerIDLogLength = 12
)

// DockerManager implements ContainerManager using Docker or Podman.
type DockerManager struct {
	// runtime is either "docker" or "podman"
	runtime string
	// hostNetwork is the network the Alyx server is reachable on.
	hostNetwork string
	// mu protects containers map.
	mu sync.RWMutex
	// containers maps container IDs to Container structs.
	containers map[string]*Container
	// httpClient is used for container communication.
	httpClient *http.Client
	// nextPort tracks the next available port for containers.
	nextPort int
	// usedPorts tracks which ports are in use.
	usedPorts map[int]bool
}

// DockerManagerConfig holds configuration for DockerManager.
type DockerManagerConfig struct {
	// Runtime is "docker" or "podman".
	Runtime string
	// HostNetwork is how containers reach the host (e.g., "host.docker.internal" for Docker Desktop).
	HostNetwork string
	// StartPort is the first port to use for container port mapping.
	StartPort int
}

// NewDockerManager creates a new DockerManager.
func NewDockerManager(config *DockerManagerConfig) (*DockerManager, error) {
	runtime := config.Runtime
	if runtime == "" {
		runtime = "docker"
	}

	// Verify the runtime is available
	if err := exec.Command(runtime, "version").Run(); err != nil {
		return nil, fmt.Errorf("container runtime %q not available: %w", runtime, err)
	}

	hostNetwork := config.HostNetwork
	if hostNetwork == "" {
		// Try to detect the host network
		hostNetwork = detectHostNetwork(runtime)
	}

	startPort := config.StartPort
	if startPort == 0 {
		startPort = 19000
	}

	return &DockerManager{
		runtime:     runtime,
		hostNetwork: hostNetwork,
		containers:  make(map[string]*Container),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		nextPort:  startPort,
		usedPorts: make(map[int]bool),
	}, nil
}

// detectHostNetwork attempts to detect the correct host network address.
func detectHostNetwork(runtime string) string {
	// For Docker Desktop, host.docker.internal works
	// For Linux Docker with host networking, localhost works
	// For Podman, host.containers.internal works

	if runtime == "podman" {
		return "host.containers.internal"
	}

	// Check if we're running on Docker Desktop (macOS/Windows)
	out, err := exec.Command(runtime, "info", "--format", "{{.OperatingSystem}}").Output()
	if err == nil {
		os := strings.ToLower(strings.TrimSpace(string(out)))
		if strings.Contains(os, "docker desktop") ||
			strings.Contains(os, "colima") ||
			strings.Contains(os, "lima") {
			return "host.docker.internal"
		}
	}

	// Default for Linux
	return "172.17.0.1"
}

// Create creates a new container for the given runtime.
func (m *DockerManager) Create(ctx context.Context, rt Runtime, config *PoolConfig) (*Container, error) {
	m.mu.Lock()
	// Find an available port
	port := m.nextPort
	for m.usedPorts[port] {
		port++
	}
	m.usedPorts[port] = true
	m.nextPort = port + 1
	m.mu.Unlock()

	containerName := fmt.Sprintf("alyx-%s-%s", rt, uuid.New().String()[:8])

	args := []string{
		"run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:%d", port, executorPort),
		"--label", fmt.Sprintf("%s=true", containerLabelKey),
		"--label", fmt.Sprintf("%s=%s", containerRuntimeLabel, rt),
	}

	// Add resource limits
	if config.MemoryLimit > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", config.MemoryLimit))
	}
	if config.CPULimit > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", config.CPULimit))
	}

	// Add image
	args = append(args, config.Image)

	log.Debug().
		Str("runtime", string(rt)).
		Str("image", config.Image).
		Int("port", port).
		Msg("Creating container")

	cmd := exec.CommandContext(ctx, m.runtime, args...) //nolint:gosec // Runtime is controlled by config
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.mu.Lock()
		delete(m.usedPorts, port)
		m.mu.Unlock()
		return nil, fmt.Errorf("failed to create container: %w: %s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))

	container := &Container{
		ID:         containerID,
		Runtime:    rt,
		State:      ContainerStateCreating,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		Port:       port,
	}

	m.mu.Lock()
	m.containers[containerID] = container
	m.mu.Unlock()

	// Wait for container to be ready
	if err := m.waitForReady(ctx, container); err != nil {
		// Clean up on failure
		_ = m.Remove(ctx, containerID)
		return nil, fmt.Errorf("container failed to become ready: %w", err)
	}

	container.State = ContainerStateReady

	log.Info().
		Str("container_id", containerID[:12]).
		Str("runtime", string(rt)).
		Int("port", port).
		Msg("Container created and ready")

	return container, nil
}

// waitForReady waits for a container to be ready to accept requests.
func (m *DockerManager) waitForReady(ctx context.Context, container *Container) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(containerReadyTimeoutSeconds * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for container to be ready")
		case <-ticker.C:
			if err := m.HealthCheck(ctx, container); err == nil {
				return nil
			}
		}
	}
}

// Start starts a stopped container.
func (m *DockerManager) Start(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, m.runtime, "start", containerID) //nolint:gosec // Runtime is controlled
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start container: %w: %s", err, string(output))
	}
	return nil
}

// Stop stops a running container.
func (m *DockerManager) Stop(ctx context.Context, containerID string) error {
	m.mu.Lock()
	if c, ok := m.containers[containerID]; ok {
		c.State = ContainerStateStopping
	}
	m.mu.Unlock()

	cmd := exec.CommandContext(ctx, m.runtime, "stop", "-t", "10", containerID) //nolint:gosec // Runtime is controlled
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop container: %w: %s", err, string(output))
	}

	m.mu.Lock()
	if c, ok := m.containers[containerID]; ok {
		c.State = ContainerStateStopped
	}
	m.mu.Unlock()

	return nil
}

// Remove removes a container.
func (m *DockerManager) Remove(ctx context.Context, containerID string) error {
	m.mu.Lock()
	container, ok := m.containers[containerID]
	if ok {
		delete(m.usedPorts, container.Port)
		delete(m.containers, containerID)
	}
	m.mu.Unlock()

	cmd := exec.CommandContext(ctx, m.runtime, "rm", "-f", containerID) //nolint:gosec // Runtime is controlled
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove container: %w: %s", err, string(output))
	}

	log.Debug().
		Str("container_id", containerID[:min(containerIDLogLength, len(containerID))]).
		Msg("Container removed")

	return nil
}

// Invoke sends a function request to a container.
func (m *DockerManager) Invoke(ctx context.Context, container *Container, req *FunctionRequest) (*FunctionResponse, error) {
	m.mu.Lock()
	container.State = ContainerStateBusy
	container.LastUsedAt = time.Now()
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		container.State = ContainerStateReady
		m.mu.Unlock()
	}()

	// Prepare the request body
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("http://localhost:%d%s", container.Port, invokePath)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke function: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var funcResp FunctionResponse
	if err := json.Unmarshal(body, &funcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if funcResp.DurationMs == 0 {
		funcResp.DurationMs = time.Since(start).Milliseconds()
	}

	return &funcResp, nil
}

// HealthCheck checks if a container is healthy.
func (m *DockerManager) HealthCheck(ctx context.Context, container *Container) error {
	url := fmt.Sprintf("http://localhost:%d%s", container.Port, healthCheckPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// List lists all containers managed by Alyx.
func (m *DockerManager) List(ctx context.Context) ([]*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	containers := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		containers = append(containers, c)
	}
	return containers, nil
}

// Close shuts down the container manager and removes all containers.
func (m *DockerManager) Close() error {
	m.mu.Lock()
	containerIDs := make([]string, 0, len(m.containers))
	for id := range m.containers {
		containerIDs = append(containerIDs, id)
	}
	m.mu.Unlock()

	ctx := context.Background()
	for _, id := range containerIDs {
		if err := m.Remove(ctx, id); err != nil {
			log.Warn().Err(err).Str("container_id", id[:min(containerIDLogLength, len(id))]).Msg("Failed to remove container during shutdown")
		}
	}

	return nil
}

// GetHostNetwork returns the host network address for containers to reach the host.
func (m *DockerManager) GetHostNetwork() string {
	return m.hostNetwork
}

// GetContainerPort returns the port a container is listening on.
func (m *DockerManager) GetContainerPort(containerID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, ok := m.containers[containerID]
	if !ok {
		return 0, fmt.Errorf("container not found: %s", containerID)
	}
	return container.Port, nil
}

// CleanupStaleContainers removes any leftover Alyx containers from previous runs.
func (m *DockerManager) CleanupStaleContainers(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, m.runtime, "ps", "-a", "-q", "--filter", fmt.Sprintf("label=%s", containerLabelKey)) //nolint:gosec // Runtime is controlled
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list stale containers: %w", err)
	}

	containerIDs := strings.Fields(string(output))
	if len(containerIDs) == 0 {
		return nil
	}

	log.Info().Int("count", len(containerIDs)).Msg("Cleaning up stale containers from previous run")

	for _, id := range containerIDs {
		rmCmd := exec.CommandContext(ctx, m.runtime, "rm", "-f", id) //nolint:gosec // Runtime is controlled
		if output, err := rmCmd.CombinedOutput(); err != nil {
			log.Warn().Err(err).Str("container_id", id[:min(containerIDLogLength, len(id))]).Str("output", string(output)).Msg("Failed to remove stale container")
		}
	}

	return nil
}

// ImageExists checks if a container image exists locally.
func (m *DockerManager) ImageExists(ctx context.Context, image string) (bool, error) {
	cmd := exec.CommandContext(ctx, m.runtime, "image", "inspect", image) //nolint:gosec // Runtime is controlled
	err := cmd.Run()
	if err != nil {
		// Image doesn't exist - this is not an error condition
		return false, nil //nolint:nilerr // Image not existing is expected
	}
	return true, nil
}

// PullImage pulls a container image.
func (m *DockerManager) PullImage(ctx context.Context, image string) error {
	log.Info().Str("image", image).Msg("Pulling container image")

	cmd := exec.CommandContext(ctx, m.runtime, "pull", image) //nolint:gosec // Runtime is controlled
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull image: %w: %s", err, string(output))
	}
	return nil
}

// GetContainerLogs retrieves logs from a container.
func (m *DockerManager) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	args := []string{"logs"}
	if tail > 0 {
		args = append(args, "--tail", strconv.Itoa(tail))
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, m.runtime, args...) //nolint:gosec // Runtime is controlled
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	return string(output), nil
}
