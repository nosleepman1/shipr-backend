package database

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type AuthProvider string

const (
	AuthProviderLocal  AuthProvider = "local"
	AuthProviderGithub AuthProvider = "github"
	AuthProviderGitlab AuthProvider = "gitlab"
	AuthProviderGoogle AuthProvider = "google"
)

func (e *AuthProvider) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = AuthProvider(s)
	case string:
		*e = AuthProvider(s)
	default:
		return fmt.Errorf("unsupported scan type for AuthProvider: %T", src)
	}
	return nil
}

func (e AuthProvider) Value() (driver.Value, error) {
	return string(e), nil
}

type BuildPackType string

const (
	BuildPackTypeNixpacks   BuildPackType = "nixpacks"
	BuildPackTypeDockerfile BuildPackType = "dockerfile"
	BuildPackTypeStatic     BuildPackType = "static"
)

func (e *BuildPackType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = BuildPackType(s)
	case string:
		*e = BuildPackType(s)
	default:
		return fmt.Errorf("unsupported scan type for BuildPackType: %T", src)
	}
	return nil
}

func (e BuildPackType) Value() (driver.Value, error) {
	return string(e), nil
}

type DeploymentStatus string

const (
	DeploymentStatusQueued    DeploymentStatus = "queued"
	DeploymentStatusCloning   DeploymentStatus = "cloning"
	DeploymentStatusBuilding  DeploymentStatus = "building"
	DeploymentStatusDeploying DeploymentStatus = "deploying"
	DeploymentStatusRunning   DeploymentStatus = "running"
	DeploymentStatusFailed    DeploymentStatus = "failed"
	DeploymentStatusCancelled DeploymentStatus = "cancelled"
)

func (e *DeploymentStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = DeploymentStatus(s)
	case string:
		*e = DeploymentStatus(s)
	default:
		return fmt.Errorf("unsupported scan type for DeploymentStatus: %T", src)
	}
	return nil
}

func (e DeploymentStatus) Value() (driver.Value, error) {
	return string(e), nil
}

type TriggerSource string

const (
	TriggerSourceManual   TriggerSource = "manual"
	TriggerSourceWebhook  TriggerSource = "webhook"
	TriggerSourceRollback TriggerSource = "rollback"
	TriggerSourceApi      TriggerSource = "api"
)

func (e *TriggerSource) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = TriggerSource(s)
	case string:
		*e = TriggerSource(s)
	default:
		return fmt.Errorf("unsupported scan type for TriggerSource: %T", src)
	}
	return nil
}

func (e TriggerSource) Value() (driver.Value, error) {
	return string(e), nil
}

type UserRole string

const (
	UserRoleSuperadmin UserRole = "superadmin"
	UserRoleUser       UserRole = "user"
)

func (e *UserRole) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = UserRole(s)
	case string:
		*e = UserRole(s)
	default:
		return fmt.Errorf("unsupported scan type for UserRole: %T", src)
	}
	return nil
}

func (e UserRole) Value() (driver.Value, error) {
	return string(e), nil
}

type WorkspaceRole string

const (
	WorkspaceRoleOwner     WorkspaceRole = "owner"
	WorkspaceRoleAdmin     WorkspaceRole = "admin"
	WorkspaceRoleDeveloper WorkspaceRole = "developer"
	WorkspaceRoleViewer    WorkspaceRole = "viewer"
)

func (e *WorkspaceRole) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = WorkspaceRole(s)
	case string:
		*e = WorkspaceRole(s)
	default:
		return fmt.Errorf("unsupported scan type for WorkspaceRole: %T", src)
	}
	return nil
}

func (e WorkspaceRole) Value() (driver.Value, error) {
	return string(e), nil
}

type User struct {
	ID              uuid.UUID          `json:"id"`
	Email           string             `json:"email"`
	EmailVerifiedAt *time.Time         `json:"email_verified_at,omitempty"`
	PasswordHash    *string            `json:"-"`
	Name            *string            `json:"name,omitempty"`
	AvatarUrl       *string            `json:"avatar_url,omitempty"`
	SystemRole      *UserRole          `json:"system_role,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

type Account struct {
	ID                uuid.UUID    `json:"id"`
	UserID            uuid.UUID    `json:"user_id"`
	Provider          AuthProvider `json:"provider"`
	ProviderAccountID string       `json:"provider_account_id"`
	AccessToken       *string      `json:"-"`
	RefreshToken      *string      `json:"-"`
	TokenExpiresAt    *time.Time   `json:"token_expires_at,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
}

type Workspace struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	OwnerID     uuid.UUID      `json:"owner_id"`
	MaxCpus     pgtype.Numeric `json:"max_cpus"`
	MaxMemoryMb *int32         `json:"max_memory_mb,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type WorkspaceMember struct {
	WorkspaceID uuid.UUID     `json:"workspace_id"`
	UserID      uuid.UUID     `json:"user_id"`
	Role        WorkspaceRole `json:"role"`
	CreatedAt   time.Time     `json:"created_at"`
}

type Project struct {
	ID          uuid.UUID `json:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	CreatedAt   time.Time `json:"created_at"`
}

type Application struct {
	ID                 uuid.UUID      `json:"id"`
	ProjectID          uuid.UUID      `json:"project_id"`
	Name               string         `json:"name"`
	GitRepositoryUrl   string         `json:"git_repository_url"`
	GitBranch          string         `json:"git_branch"`
	BuildPack          BuildPackType  `json:"build_pack"`
	DockerfilePath     *string        `json:"dockerfile_path,omitempty"`
	BaseDirectory      *string        `json:"base_directory,omitempty"`
	BuildCommand       *string        `json:"build_command,omitempty"`
	StartCommand       *string        `json:"start_command,omitempty"`
	InternalPort       int32          `json:"internal_port"`
	CpuLimit           pgtype.Numeric `json:"cpu_limit"`
	MemoryLimitMb      *int32         `json:"memory_limit_mb,omitempty"`
	Status             *string        `json:"status,omitempty"`
	ActiveDeploymentID *uuid.UUID     `json:"active_deployment_id,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type EnvironmentVariable struct {
	ID             uuid.UUID `json:"id"`
	ApplicationID  uuid.UUID `json:"application_id"`
	Key            string    `json:"key"`
	ValueEncrypted string    `json:"value_encrypted"`
	IsSecret       *bool     `json:"is_secret,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type Deployment struct {
	ID             uuid.UUID        `json:"id"`
	ApplicationID  uuid.UUID        `json:"application_id"`
	TriggeredBy    *uuid.UUID       `json:"triggered_by,omitempty"`
	TriggerSource  TriggerSource    `json:"trigger_source"`
	Status         DeploymentStatus `json:"status"`
	CommitHash     *string          `json:"commit_hash,omitempty"`
	CommitMessage  *string          `json:"commit_message,omitempty"`
	CommitAuthor   *string          `json:"commit_author,omitempty"`
	DockerImageTag *string          `json:"docker_image_tag,omitempty"`
	ContainerID    *string          `json:"container_id,omitempty"`
	StartedAt      *time.Time       `json:"started_at,omitempty"`
	FinishedAt     *time.Time       `json:"finished_at,omitempty"`
	DurationMs     *int32           `json:"duration_ms,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

type DeploymentLog struct {
	ID           int64     `json:"id"`
	DeploymentID uuid.UUID `json:"deployment_id"`
	Stream       *string   `json:"stream,omitempty"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}

type Domain struct {
	ID            uuid.UUID `json:"id"`
	ApplicationID uuid.UUID `json:"application_id"`
	Domain        string    `json:"domain"`
	IsSubdomain   *bool     `json:"is_subdomain,omitempty"`
	SslStatus     *string   `json:"ssl_status,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
