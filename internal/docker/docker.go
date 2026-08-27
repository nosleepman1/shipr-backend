package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type Client struct {
	cli     *client.Client
	network string
}

type ContainerRunConfig struct {
	Name           string
	ImageTag       string
	NetworkName    string
	InternalPort   int32
	CPULimit       float64 // ex: 0.5 CPUs
	MemoryLimitMB  int64   // ex: 512 MB
	EnvVars        []string
	Labels         map[string]string
	StartCommand   []string
}

type ContainerStats struct {
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	MemoryUsageMB   float64 `json:"memory_usage_mb"`
	MemoryLimitMB   float64 `json:"memory_limit_mb"`
	MemoryPercent   float64 `json:"memory_percent"`
	Timestamp       time.Time `json:"timestamp"`
}

func NewClient(defaultNetwork string) (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Client{
		cli:     cli,
		network: defaultNetwork,
	}, nil
}

// EnsureNetworkExists creates the docker network if it doesn't exist already
func (c *Client) EnsureNetworkExists(ctx context.Context, networkName string) error {
	networks, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}

	for _, n := range networks {
		if n.Name == networkName {
			return nil
		}
	}

	_, err = c.cli.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "bridge",
	})
	return err
}

// CreateAndStartContainer creates a container with resource constraints and connects it to the network
func (c *Client) CreateAndStartContainer(ctx context.Context, cfg ContainerRunConfig) (string, error) {
	netName := cfg.NetworkName
	if netName == "" {
		netName = c.network
	}
	_ = c.EnsureNetworkExists(ctx, netName)

	// Resource limits
	resources := container.Resources{}
	if cfg.CPULimit > 0 {
		resources.NanoCPUs = int64(cfg.CPULimit * 1e9)
	}
	if cfg.MemoryLimitMB > 0 {
		resources.Memory = cfg.MemoryLimitMB * 1024 * 1024
	}

	hostConfig := &container.HostConfig{
		Resources: resources,
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
		NetworkMode: container.NetworkMode(netName),
	}

	containerConfig := &container.Config{
		Image:  cfg.ImageTag,
		Env:    cfg.EnvVars,
		Labels: cfg.Labels,
	}
	if len(cfg.StartCommand) > 0 {
		containerConfig.Cmd = cfg.StartCommand
	}

	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			netName: {
				NetworkID: netName,
			},
		},
	}

	resp, err := c.cli.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		networkConfig,
		nil,
		cfg.Name,
	)
	if err != nil {
		return "", fmt.Errorf("container create failed: %w", err)
	}

	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return resp.ID, fmt.Errorf("container start failed: %w", err)
	}

	return resp.ID, nil
}

// GetContainerIP returns the internal IP address of the container on the target network
func (c *Client) GetContainerIP(ctx context.Context, containerID string, networkName string) (string, error) {
	if networkName == "" {
		networkName = c.network
	}

	inspect, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	if netData, ok := inspect.NetworkSettings.Networks[networkName]; ok && netData.IPAddress != "" {
		return netData.IPAddress, nil
	}

	// Fallback to primary IP address
	if inspect.NetworkSettings.IPAddress != "" {
		return inspect.NetworkSettings.IPAddress, nil
	}

	return "", fmt.Errorf("container %s has no assigned IP address on network %s", containerID, networkName)
}

// WaitForHealthcheck polls the container HTTP endpoint until status 200/healthy or timeout
func (c *Client) WaitForHealthcheck(ctx context.Context, ip string, port int32, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	url := fmt.Sprintf("http://%s:%d/", ip, port)

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				// Any response under 500 means the server is up and listening
				if resp.StatusCode < 500 {
					return nil
				}
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	return fmt.Errorf("healthcheck timed out after %s for %s", timeout, url)
}

// GetContainerStats retrieves current CPU and memory metrics
func (c *Client) GetContainerStats(ctx context.Context, containerID string) (*ContainerStats, error) {
	statResp, err := c.cli.ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		return nil, err
	}
	defer statResp.Body.Close()

	bodyBytes, err := io.ReadAll(statResp.Body)
	if err != nil {
		return nil, err
	}

	var data struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs     uint64 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
	}

	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, err
	}

	// Calculate CPU percentage
	cpuPercent := 0.0
	cpuDelta := float64(data.CPUStats.CPUUsage.TotalUsage) - float64(data.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(data.CPUStats.SystemCPUUsage) - float64(data.PreCPUStats.SystemCPUUsage)
	onlineCPUs := float64(data.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}

	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	// Calculate Memory
	memUsageMB := float64(data.MemoryStats.Usage) / (1024 * 1024)
	memLimitMB := float64(data.MemoryStats.Limit) / (1024 * 1024)
	memPercent := 0.0
	if memLimitMB > 0 {
		memPercent = (memUsageMB / memLimitMB) * 100.0
	}

	return &ContainerStats{
		CPUUsagePercent: cpuPercent,
		MemoryUsageMB:   memUsageMB,
		MemoryLimitMB:   memLimitMB,
		MemoryPercent:   memPercent,
		Timestamp:       time.Now(),
	}, nil
}

// StopAndRemoveContainer gracefully stops and deletes a container
func (c *Client) StopAndRemoveContainer(ctx context.Context, containerID string) error {
	if containerID == "" {
		return nil
	}

	timeoutSec := 10
	_ = c.cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeoutSec,
	})

	return c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
}

// RemoveImage removes a Docker image
func (c *Client) RemoveImage(ctx context.Context, imageTag string) error {
	if imageTag == "" {
		return nil
	}
	_, err := c.cli.ImageRemove(ctx, imageTag, image.RemoveOptions{
		Force:         true,
		PruneChildren: true,
	})
	return err
}
