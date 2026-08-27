package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"shipr-backend/internal/caddy"
	"shipr-backend/internal/config"
	"shipr-backend/internal/crypto"
	"shipr-backend/internal/database"
	"shipr-backend/internal/docker"
	"shipr-backend/internal/tasks"
)

type Worker struct {
	cfg       *config.Config
	db        *database.Queries
	dockerCli *docker.Client
	caddyCli  *caddy.Client
	encryptor *crypto.Encryptor
	rdb       *redis.Client
}

func NewWorker(
	cfg *config.Config,
	db *database.Queries,
	dockerCli *docker.Client,
	caddyCli *caddy.Client,
	encryptor *crypto.Encryptor,
	rdb *redis.Client,
) *Worker {
	return &Worker{
		cfg:       cfg,
		db:        db,
		dockerCli: dockerCli,
		caddyCli:  caddyCli,
		encryptor: encryptor,
		rdb:       rdb,
	}
}

// LogBroadcaster streams logs to Redis Pub/Sub and buffers them for PostgreSQL batch insertion
type LogBroadcaster struct {
	deploymentID uuid.UUID
	rdb          *redis.Client
	db           *database.Queries
	mu           sync.Mutex
	buffer       []string
	ctx          context.Context
}

func newLogBroadcaster(ctx context.Context, deploymentID uuid.UUID, rdb *redis.Client, db *database.Queries) *LogBroadcaster {
	return &LogBroadcaster{
		deploymentID: deploymentID,
		rdb:          rdb,
		db:           db,
		buffer:       make([]string, 0, 100),
		ctx:          ctx,
	}
}

func (lb *LogBroadcaster) Log(stream, line string) {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		return
	}

	// 1. Publish immediately to Redis Pub/Sub for SSE live streaming
	channel := fmt.Sprintf("logs:%s", lb.deploymentID.String())
	payload, _ := json.Marshal(map[string]string{
		"stream":    stream,
		"message":   trimmed,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	_ = lb.rdb.Publish(lb.ctx, channel, string(payload)).Err()

	// 2. Persist to database
	lb.mu.Lock()
	lb.buffer = append(lb.buffer, trimmed)
	shouldFlush := len(lb.buffer) >= 20
	lb.mu.Unlock()

	if shouldFlush {
		lb.Flush(stream)
	}
}

func (lb *LogBroadcaster) Flush(stream string) {
	lb.mu.Lock()
	if len(lb.buffer) == 0 {
		lb.mu.Unlock()
		return
	}
	toFlush := lb.buffer
	lb.buffer = make([]string, 0, 100)
	lb.mu.Unlock()

	for _, msg := range toFlush {
		streamVal := stream
		_, _ = lb.db.InsertDeploymentLog(lb.ctx, lb.deploymentID, &streamVal, msg)
	}
}

func (w *Worker) ProcessDeployTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.DeployApplicationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("invalid task payload: %w", err)
	}

	deploymentID, err := uuid.Parse(p.DeploymentID)
	if err != nil {
		return fmt.Errorf("invalid deployment id: %w", err)
	}

	appID, err := uuid.Parse(p.ApplicationID)
	if err != nil {
		return fmt.Errorf("invalid application id: %w", err)
	}

	app, err := w.db.GetApplicationByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("failed to fetch application: %w", err)
	}

	logger := newLogBroadcaster(ctx, deploymentID, w.rdb, w.db)
	defer logger.Flush("stdout")

	logger.Log("stdout", fmt.Sprintf("🚀 Starting build & deployment for application: %s", app.Name))

	// STEP 1: Update status to 'cloning'
	_, _ = w.db.UpdateDeploymentStatus(ctx, deploymentID, database.DeploymentStatusCloning)
	runningStatus := "building"
	_, _ = w.db.UpdateApplicationStatus(ctx, appID, &runningStatus)

	// Create isolated build directory
	buildDir := filepath.Join(w.cfg.BuildStorageDir, deploymentID.String())
	_ = os.RemoveAll(buildDir)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return w.failDeployment(ctx, deploymentID, appID, logger, fmt.Sprintf("Failed to create build directory: %v", err))
	}
	defer os.RemoveAll(buildDir)

	// STEP 2: Git Clone
	branch := app.GitBranch
	if branch == "" {
		branch = "main"
	}
	logger.Log("stdout", fmt.Sprintf("📥 Cloning %s (branch: %s)...", app.GitRepositoryUrl, branch))

	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, app.GitRepositoryUrl, buildDir)
	cloneOutput, err := cloneCmd.CombinedOutput()
	if err != nil {
		return w.failDeployment(ctx, deploymentID, appID, logger, fmt.Sprintf("Git clone failed: %s - %v", string(cloneOutput), err))
	}

	// Extract commit metadata
	commitHashCmd := exec.CommandContext(ctx, "git", "-C", buildDir, "rev-parse", "HEAD")
	commitHashOut, _ := commitHashCmd.Output()
	commitHash := strings.TrimSpace(string(commitHashOut))
	shortHash := commitHash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	commitMsgCmd := exec.CommandContext(ctx, "git", "-C", buildDir, "log", "-1", "--pretty=%B")
	commitMsgOut, _ := commitMsgCmd.Output()
	commitMsg := strings.TrimSpace(string(commitMsgOut))

	commitAuthorCmd := exec.CommandContext(ctx, "git", "-C", buildDir, "log", "-1", "--pretty=%an")
	commitAuthorOut, _ := commitAuthorCmd.Output()
	commitAuthor := strings.TrimSpace(string(commitAuthorOut))

	_, _ = w.db.UpdateDeploymentCommitInfo(ctx, deploymentID, &commitHash, &commitMsg, &commitAuthor)
	logger.Log("stdout", fmt.Sprintf("✅ Commit: %s (%s) by %s", shortHash, commitMsg, commitAuthor))

	// STEP 3: Environment Variables Preparation
	envVars, err := w.db.ListEnvironmentVariablesByAppID(ctx, appID)
	if err != nil {
		logger.Log("stderr", fmt.Sprintf("Warning: failed to fetch env vars: %v", err))
	}

	var envFileContent strings.Builder
	var containerEnvList []string

	for _, ev := range envVars {
		val := ev.ValueEncrypted
		decrypted, err := w.encryptor.Decrypt(val)
		if err == nil {
			val = decrypted
		}
		envFileContent.WriteString(fmt.Sprintf("%s=%s\n", ev.Key, val))
		containerEnvList = append(containerEnvList, fmt.Sprintf("%s=%s", ev.Key, val))
	}

	envFilePath := filepath.Join(buildDir, ".env")
	_ = os.WriteFile(envFilePath, []byte(envFileContent.String()), 0600)
	logger.Log("stdout", fmt.Sprintf("🔒 Injected %d environment variables", len(envVars)))

	// STEP 4: Build Execution (Nixpacks or Docker)
	_, _ = w.db.UpdateDeploymentStatus(ctx, deploymentID, database.DeploymentStatusBuilding)
	imageTag := fmt.Sprintf("shipr-%s:%s", appID.String()[:8], shortHash)

	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	if app.DockerfilePath != nil && *app.DockerfilePath != "" {
		dockerfilePath = filepath.Join(buildDir, *app.DockerfilePath)
	}

	hasDockerfile := false
	if _, err := os.Stat(dockerfilePath); err == nil {
		hasDockerfile = true
	}

	var buildCmd *exec.Cmd
	if hasDockerfile || app.BuildPack == database.BuildPackTypeDockerfile {
		logger.Log("stdout", "🐳 Dockerfile detected. Building with docker...")
		buildCmd = exec.CommandContext(ctx, "docker", "build", "-t", imageTag, buildDir)
	} else {
		// Try Nixpacks CLI first
		if _, err := exec.LookPath("nixpacks"); err == nil {
			logger.Log("stdout", "📦 Building with Nixpacks...")
			buildCmd = exec.CommandContext(ctx, "nixpacks", "build", buildDir, "--name", imageTag)
		} else {
			logger.Log("stdout", "⚠️ Nixpacks CLI not found on host, falling back to docker build...")
			buildCmd = exec.CommandContext(ctx, "docker", "build", "-t", imageTag, buildDir)
		}
	}

	// Stream stdout & stderr live
	stdoutPipe, err := buildCmd.StdoutPipe()
	if err != nil {
		return w.failDeployment(ctx, deploymentID, appID, logger, fmt.Sprintf("Failed to open stdout pipe: %v", err))
	}
	stderrPipe, err := buildCmd.StderrPipe()
	if err != nil {
		return w.failDeployment(ctx, deploymentID, appID, logger, fmt.Sprintf("Failed to open stderr pipe: %v", err))
	}

	if err := buildCmd.Start(); err != nil {
		return w.failDeployment(ctx, deploymentID, appID, logger, fmt.Sprintf("Build start error: %v", err))
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			logger.Log("stdout", scanner.Text())
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			logger.Log("stderr", scanner.Text())
		}
	}()

	wg.Wait()
	if err := buildCmd.Wait(); err != nil {
		return w.failDeployment(ctx, deploymentID, appID, logger, fmt.Sprintf("Build failed with error: %v", err))
	}

	logger.Log("stdout", fmt.Sprintf("✅ Image built successfully: %s", imageTag))

	// STEP 5: Deploy Container via Docker Go SDK
	_, _ = w.db.UpdateDeploymentStatus(ctx, deploymentID, database.DeploymentStatusDeploying)
	containerName := fmt.Sprintf("app-%s-%s", appID.String()[:8], shortHash)

	cpuLimitFloat := 0.5
	if app.CpuLimit.Valid {
		// Convert pgtype.Numeric to float64 if needed, default to 0.5
		cpuLimitFloat = 0.5
	}
	memLimitMB := int64(512)
	if app.MemoryLimitMb != nil && *app.MemoryLimitMb > 0 {
		memLimitMB = int64(*app.MemoryLimitMb)
	}

	logger.Log("stdout", fmt.Sprintf("🚢 Starting container %s (CPU: %.2f, RAM: %dMB)...", containerName, cpuLimitFloat, memLimitMB))

	containerID, err := w.dockerCli.CreateAndStartContainer(ctx, docker.ContainerRunConfig{
		Name:          containerName,
		ImageTag:      imageTag,
		NetworkName:   w.cfg.DockerNetwork,
		InternalPort:  app.InternalPort,
		CPULimit:      cpuLimitFloat,
		MemoryLimitMB: memLimitMB,
		EnvVars:       containerEnvList,
		Labels: map[string]string{
			"shipr.app_id":        appID.String(),
			"shipr.deployment_id": deploymentID.String(),
		},
	})
	if err != nil {
		return w.failDeployment(ctx, deploymentID, appID, logger, fmt.Sprintf("Failed to run container: %v", err))
	}

	_, _ = w.db.UpdateDeploymentContainer(ctx, deploymentID, &imageTag, &containerID)

	// STEP 6: Inspect IP & Dynamic Caddy Ingress Configuration
	containerIP, err := w.dockerCli.GetContainerIP(ctx, containerID, w.cfg.DockerNetwork)
	if err != nil {
		_ = w.dockerCli.StopAndRemoveContainer(ctx, containerID)
		return w.failDeployment(ctx, deploymentID, appID, logger, fmt.Sprintf("Failed to get container IP: %v", err))
	}
	logger.Log("stdout", fmt.Sprintf("🌐 Container assigned IP: %s (internal port: %d)", containerIP, app.InternalPort))

	// Collect hostnames to route
	var hostnames []string
	defaultSubdomain := fmt.Sprintf("%s.%s", strings.ToLower(app.Name), w.cfg.BaseDomain)
	hostnames = append(hostnames, defaultSubdomain)

	// Fetch custom domains from DB
	domains, _ := w.db.ListDomainsByAppID(ctx, appID)
	for _, d := range domains {
		hostnames = append(hostnames, d.Domain)
	}

	logger.Log("stdout", fmt.Sprintf("⚙️ Updating Caddy routing for domains: %s -> %s:%d", strings.Join(hostnames, ", "), containerIP, app.InternalPort))
	if err := w.caddyCli.UpsertAppRoute(ctx, appID.String(), hostnames, containerIP, app.InternalPort); err != nil {
		logger.Log("stderr", fmt.Sprintf("Warning: Caddy route update failed: %v", err))
	}

	// STEP 7: Healthcheck & Zero-Downtime Switchover
	logger.Log("stdout", "🩺 Performing healthcheck...")
	healthCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := w.dockerCli.WaitForHealthcheck(healthCtx, containerIP, app.InternalPort, 30*time.Second); err != nil {
		logger.Log("stderr", fmt.Sprintf("Healthcheck warning (service might not have HTTP root endpoint): %v", err))
	} else {
		logger.Log("stdout", "✅ Healthcheck passed! Service is responding.")
	}

	// Teardown previous active deployment container if present
	if app.ActiveDeploymentID != nil {
		oldDep, err := w.db.GetDeploymentByID(ctx, *app.ActiveDeploymentID)
		if err == nil && oldDep.ContainerID != nil && *oldDep.ContainerID != containerID {
			logger.Log("stdout", fmt.Sprintf("🧹 Gracefully stopping previous container %s...", (*oldDep.ContainerID)[:12]))
			_ = w.dockerCli.StopAndRemoveContainer(ctx, *oldDep.ContainerID)
		}
	}

	// Finalize status
	_, _ = w.db.FinishDeployment(ctx, deploymentID, database.DeploymentStatusRunning)
	runningAppStatus := "running"
	_, _ = w.db.UpdateActiveDeployment(ctx, appID, &deploymentID, &runningAppStatus)

	logger.Log("stdout", "🎉 Deployment completed successfully and is now LIVE!")
	return nil
}

func (w *Worker) failDeployment(ctx context.Context, deploymentID, appID uuid.UUID, logger *LogBroadcaster, reason string) error {
	log.Printf("[DEPLOYMENT ERROR] App: %s, Dep: %s: %s", appID, deploymentID, reason)
	logger.Log("stderr", fmt.Sprintf("❌ DEPLOYMENT FAILED: %s", reason))
	logger.Flush("stderr")

	_, _ = w.db.FinishDeployment(ctx, deploymentID, database.DeploymentStatusFailed)
	failedStatus := "failed"
	_, _ = w.db.UpdateApplicationStatus(ctx, appID, &failedStatus)

	return fmt.Errorf("%s", reason)
}
