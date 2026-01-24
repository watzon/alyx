package functions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Pool manages a pool of warm containers for a specific runtime.
type Pool struct {
	runtime    Runtime
	config     *PoolConfig
	manager    *DockerManager
	containers []*Container
	mu         sync.RWMutex
	available  chan *Container
	closing    bool
	wg         sync.WaitGroup
}

// PoolManager manages multiple pools, one per runtime.
type PoolManager struct {
	manager *DockerManager
	pools   map[Runtime]*Pool
	configs map[Runtime]*PoolConfig
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// PoolManagerConfig holds configuration for PoolManager.
type PoolManagerConfig struct {
	// ContainerRuntime is "docker" or "podman".
	ContainerRuntime string
	// HostNetwork is how containers reach the host.
	HostNetwork string
	// StartPort is the first port to use for containers.
	StartPort int
	// RuntimeConfigs maps runtime to pool configuration.
	RuntimeConfigs map[Runtime]*PoolConfig
}

// NewPoolManager creates a new PoolManager.
func NewPoolManager(config *PoolManagerConfig) (*PoolManager, error) {
	dockerManager, err := NewDockerManager(&DockerManagerConfig{
		Runtime:     config.ContainerRuntime,
		HostNetwork: config.HostNetwork,
		StartPort:   config.StartPort,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create container manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	pm := &PoolManager{
		manager: dockerManager,
		pools:   make(map[Runtime]*Pool),
		configs: config.RuntimeConfigs,
		ctx:     ctx,
		cancel:  cancel,
	}

	return pm, nil
}

// Start initializes the pool manager and starts maintaining warm pools.
func (pm *PoolManager) Start(ctx context.Context) error {
	// Clean up any stale containers from previous runs
	if err := pm.manager.CleanupStaleContainers(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to cleanup stale containers")
	}

	// Initialize pools for each runtime
	for runtime, config := range pm.configs {
		if config.Image == "" {
			log.Debug().Str("runtime", string(runtime)).Msg("Skipping runtime with no image configured")
			continue
		}

		// Check if image exists, pull if not
		exists, err := pm.manager.ImageExists(ctx, config.Image)
		if err != nil {
			log.Warn().Err(err).Str("image", config.Image).Msg("Failed to check if image exists")
		}
		if !exists {
			if err := pm.manager.PullImage(ctx, config.Image); err != nil {
				log.Warn().Err(err).Str("image", config.Image).Msg("Failed to pull image, pool will not be started")
				continue
			}
		}

		pool := &Pool{
			runtime:    runtime,
			config:     config,
			manager:    pm.manager,
			containers: make([]*Container, 0, config.MaxInstances),
			available:  make(chan *Container, config.MaxInstances),
		}

		pm.mu.Lock()
		pm.pools[runtime] = pool
		pm.mu.Unlock()

		// Start the pool maintenance goroutine
		pool.wg.Add(1)
		go pool.maintain(pm.ctx)

		log.Info().
			Str("runtime", string(runtime)).
			Int("min_warm", config.MinWarm).
			Int("max_instances", config.MaxInstances).
			Msg("Pool initialized")
	}

	return nil
}

// maintain keeps the pool at the configured minimum warm instances.
func (p *Pool) maintain(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	idleTicker := time.NewTicker(10 * time.Second)
	defer idleTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.ensureMinWarm(ctx)
		case <-idleTicker.C:
			p.cleanupIdle(ctx)
		}
	}
}

// ensureMinWarm ensures the pool has at least MinWarm ready containers.
func (p *Pool) ensureMinWarm(ctx context.Context) {
	p.mu.RLock()
	if p.closing {
		p.mu.RUnlock()
		return
	}

	readyCount := 0
	for _, c := range p.containers {
		if c.State == ContainerStateReady {
			readyCount++
		}
	}
	totalCount := len(p.containers)
	p.mu.RUnlock()

	// Need to create more containers
	neededCount := p.config.MinWarm - readyCount
	if neededCount <= 0 {
		return
	}

	// But don't exceed max instances
	availableSlots := p.config.MaxInstances - totalCount
	if availableSlots <= 0 {
		return
	}

	createCount := min(neededCount, availableSlots)
	for range createCount {
		container, err := p.manager.Create(ctx, p.runtime, p.config)
		if err != nil {
			log.Error().Err(err).Str("runtime", string(p.runtime)).Msg("Failed to create warm container")
			continue
		}

		p.mu.Lock()
		p.containers = append(p.containers, container)
		p.mu.Unlock()

		// Add to available channel
		select {
		case p.available <- container:
		default:
			// Channel full, container is still tracked
		}
	}
}

// cleanupIdle removes containers that have been idle for too long.
func (p *Pool) cleanupIdle(ctx context.Context) {
	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		return
	}

	now := time.Now()
	toRemove := make([]string, 0)

	// Find idle containers beyond min_warm
	readyCount := 0
	for _, c := range p.containers {
		if c.State == ContainerStateReady {
			readyCount++
		}
	}

	// Only cleanup if we have more than min_warm
	if readyCount <= p.config.MinWarm {
		p.mu.Unlock()
		return
	}

	for _, c := range p.containers {
		if c.State == ContainerStateReady && readyCount > p.config.MinWarm {
			if now.Sub(c.LastUsedAt) > p.config.IdleTimeout {
				toRemove = append(toRemove, c.ID)
				readyCount--
			}
		}
	}
	p.mu.Unlock()

	for _, id := range toRemove {
		if err := p.manager.Remove(ctx, id); err != nil {
			log.Warn().Err(err).Str("container_id", id[:min(containerIDLogLength, len(id))]).Msg("Failed to remove idle container")
		}

		p.mu.Lock()
		for i, c := range p.containers {
			if c.ID == id {
				p.containers = append(p.containers[:i], p.containers[i+1:]...)
				break
			}
		}
		p.mu.Unlock()
	}
}

// Acquire gets an available container from the pool, creating one if necessary.
func (p *Pool) Acquire(ctx context.Context) (*Container, error) {
	p.mu.RLock()
	if p.closing {
		p.mu.RUnlock()
		return nil, fmt.Errorf("pool is closing")
	}
	p.mu.RUnlock()

	// Try to get from available channel first (non-blocking)
	select {
	case container := <-p.available:
		if container.State == ContainerStateReady {
			return container, nil
		}
		// Container not ready, try to find another
	default:
		// No available containers
	}

	// Look for any ready container
	p.mu.Lock()
	for _, c := range p.containers {
		if c.State == ContainerStateReady {
			p.mu.Unlock()
			return c, nil
		}
	}

	// Check if we can create a new one
	if len(p.containers) >= p.config.MaxInstances {
		p.mu.Unlock()
		// Wait for one to become available
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case container := <-p.available:
			if container.State == ContainerStateReady {
				return container, nil
			}
			return nil, fmt.Errorf("acquired container is not ready")
		}
	}
	p.mu.Unlock()

	// Create a new container
	container, err := p.manager.Create(ctx, p.runtime, p.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	p.mu.Lock()
	p.containers = append(p.containers, container)
	p.mu.Unlock()

	return container, nil
}

// Release returns a container to the pool.
func (p *Pool) Release(container *Container) {
	container.State = ContainerStateReady
	container.LastUsedAt = time.Now()

	select {
	case p.available <- container:
	default:
		// Channel full, container is still tracked
	}
}

// Invoke acquires a container, invokes the function, and releases the container.
func (p *Pool) Invoke(ctx context.Context, req *FunctionRequest) (*FunctionResponse, error) {
	container, err := p.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire container: %w", err)
	}
	defer p.Release(container)

	return p.manager.Invoke(ctx, container, req)
}

// GetPool returns the pool for a given runtime.
func (pm *PoolManager) GetPool(runtime Runtime) (*Pool, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pool, ok := pm.pools[runtime]
	if !ok {
		return nil, fmt.Errorf("no pool for runtime: %s", runtime)
	}
	return pool, nil
}

// Invoke executes a function on the appropriate pool.
func (pm *PoolManager) Invoke(ctx context.Context, runtime Runtime, req *FunctionRequest) (*FunctionResponse, error) {
	pool, err := pm.GetPool(runtime)
	if err != nil {
		return nil, err
	}
	return pool.Invoke(ctx, req)
}

// GetHostNetwork returns the host network address.
func (pm *PoolManager) GetHostNetwork() string {
	return pm.manager.GetHostNetwork()
}

// Stats returns statistics about all pools.
func (pm *PoolManager) Stats() map[Runtime]PoolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := make(map[Runtime]PoolStats)
	for runtime, pool := range pm.pools {
		pool.mu.RLock()
		var ready, busy, total int
		for _, c := range pool.containers {
			total++
			switch c.State {
			case ContainerStateReady:
				ready++
			case ContainerStateBusy:
				busy++
			}
		}
		pool.mu.RUnlock()

		stats[runtime] = PoolStats{
			Ready: ready,
			Busy:  busy,
			Total: total,
		}
	}
	return stats
}

// PoolStats contains statistics about a pool.
type PoolStats struct {
	Ready int `json:"ready"`
	Busy  int `json:"busy"`
	Total int `json:"total"`
}

// Close shuts down all pools and the container manager.
func (pm *PoolManager) Close() error {
	pm.cancel()

	pm.mu.Lock()
	for _, pool := range pm.pools {
		pool.mu.Lock()
		pool.closing = true
		close(pool.available)
		pool.mu.Unlock()
		pool.wg.Wait()
	}
	pm.mu.Unlock()

	return pm.manager.Close()
}
