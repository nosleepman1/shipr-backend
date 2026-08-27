package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ============================================================================
// USERS & AUTH
// ============================================================================

const createUser = `-- name: CreateUser :one
INSERT INTO users (email, password_hash, name, avatar_url, system_role)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, email_verified_at, password_hash, name, avatar_url, system_role, created_at, updated_at
`

type CreateUserParams struct {
	Email        string    `json:"email"`
	PasswordHash *string   `json:"password_hash"`
	Name         *string   `json:"name"`
	AvatarUrl    *string   `json:"avatar_url"`
	SystemRole   *UserRole `json:"system_role"`
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRow(ctx, createUser,
		arg.Email,
		arg.PasswordHash,
		arg.Name,
		arg.AvatarUrl,
		arg.SystemRole,
	)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Email,
		&i.EmailVerifiedAt,
		&i.PasswordHash,
		&i.Name,
		&i.AvatarUrl,
		&i.SystemRole,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getUserByEmail = `-- name: GetUserByEmail :one
SELECT id, email, email_verified_at, password_hash, name, avatar_url, system_role, created_at, updated_at FROM users
WHERE email = $1 LIMIT 1
`

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRow(ctx, getUserByEmail, email)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Email,
		&i.EmailVerifiedAt,
		&i.PasswordHash,
		&i.Name,
		&i.AvatarUrl,
		&i.SystemRole,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getUserByID = `-- name: GetUserByID :one
SELECT id, email, email_verified_at, password_hash, name, avatar_url, system_role, created_at, updated_at FROM users
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	row := q.db.QueryRow(ctx, getUserByID, id)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Email,
		&i.EmailVerifiedAt,
		&i.PasswordHash,
		&i.Name,
		&i.AvatarUrl,
		&i.SystemRole,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

// ============================================================================
// WORKSPACES & MEMBERS
// ============================================================================

const createWorkspace = `-- name: CreateWorkspace :one
INSERT INTO workspaces (name, slug, owner_id, max_cpus, max_memory_mb)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, slug, owner_id, max_cpus, max_memory_mb, created_at
`

type CreateWorkspaceParams struct {
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	OwnerID     uuid.UUID      `json:"owner_id"`
	MaxCpus     pgtype.Numeric `json:"max_cpus"`
	MaxMemoryMb *int32         `json:"max_memory_mb"`
}

func (q *Queries) CreateWorkspace(ctx context.Context, arg CreateWorkspaceParams) (Workspace, error) {
	row := q.db.QueryRow(ctx, createWorkspace,
		arg.Name,
		arg.Slug,
		arg.OwnerID,
		arg.MaxCpus,
		arg.MaxMemoryMb,
	)
	var i Workspace
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Slug,
		&i.OwnerID,
		&i.MaxCpus,
		&i.MaxMemoryMb,
		&i.CreatedAt,
	)
	return i, err
}

const getWorkspaceByID = `-- name: GetWorkspaceByID :one
SELECT id, name, slug, owner_id, max_cpus, max_memory_mb, created_at FROM workspaces
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetWorkspaceByID(ctx context.Context, id uuid.UUID) (Workspace, error) {
	row := q.db.QueryRow(ctx, getWorkspaceByID, id)
	var i Workspace
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Slug,
		&i.OwnerID,
		&i.MaxCpus,
		&i.MaxMemoryMb,
		&i.CreatedAt,
	)
	return i, err
}

const getWorkspaceBySlug = `-- name: GetWorkspaceBySlug :one
SELECT id, name, slug, owner_id, max_cpus, max_memory_mb, created_at FROM workspaces
WHERE slug = $1 LIMIT 1
`

func (q *Queries) GetWorkspaceBySlug(ctx context.Context, slug string) (Workspace, error) {
	row := q.db.QueryRow(ctx, getWorkspaceBySlug, slug)
	var i Workspace
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Slug,
		&i.OwnerID,
		&i.MaxCpus,
		&i.MaxMemoryMb,
		&i.CreatedAt,
	)
	return i, err
}

const listWorkspacesForUser = `-- name: ListWorkspacesForUser :many
SELECT w.id, w.name, w.slug, w.owner_id, w.max_cpus, w.max_memory_mb, w.created_at, wm.role as member_role
FROM workspaces w
INNER JOIN workspace_members wm ON w.id = wm.workspace_id
WHERE wm.user_id = $1
ORDER BY w.created_at DESC
`

type ListWorkspacesForUserRow struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	OwnerID     uuid.UUID      `json:"owner_id"`
	MaxCpus     pgtype.Numeric `json:"max_cpus"`
	MaxMemoryMb *int32         `json:"max_memory_mb"`
	CreatedAt   time.Time      `json:"created_at"`
	MemberRole  WorkspaceRole  `json:"member_role"`
}

func (q *Queries) ListWorkspacesForUser(ctx context.Context, userID uuid.UUID) ([]ListWorkspacesForUserRow, error) {
	rows, err := q.db.Query(ctx, listWorkspacesForUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListWorkspacesForUserRow
	for rows.Next() {
		var i ListWorkspacesForUserRow
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Slug,
			&i.OwnerID,
			&i.MaxCpus,
			&i.MaxMemoryMb,
			&i.CreatedAt,
			&i.MemberRole,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const addWorkspaceMember = `-- name: AddWorkspaceMember :one
INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING workspace_id, user_id, role, created_at
`

func (q *Queries) AddWorkspaceMember(ctx context.Context, workspaceID, userID uuid.UUID, role WorkspaceRole) (WorkspaceMember, error) {
	row := q.db.QueryRow(ctx, addWorkspaceMember, workspaceID, userID, role)
	var i WorkspaceMember
	err := row.Scan(
		&i.WorkspaceID,
		&i.UserID,
		&i.Role,
		&i.CreatedAt,
	)
	return i, err
}

const getWorkspaceMemberRole = `-- name: GetWorkspaceMemberRole :one
SELECT role FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2 LIMIT 1
`

func (q *Queries) GetWorkspaceMemberRole(ctx context.Context, workspaceID, userID uuid.UUID) (WorkspaceRole, error) {
	row := q.db.QueryRow(ctx, getWorkspaceMemberRole, workspaceID, userID)
	var role WorkspaceRole
	err := row.Scan(&role)
	return role, err
}

// ============================================================================
// PROJECTS
// ============================================================================

const createProject = `-- name: CreateProject :one
INSERT INTO projects (workspace_id, name, slug)
VALUES ($1, $2, $3)
RETURNING id, workspace_id, name, slug, created_at
`

func (q *Queries) CreateProject(ctx context.Context, workspaceID uuid.UUID, name, slug string) (Project, error) {
	row := q.db.QueryRow(ctx, createProject, workspaceID, name, slug)
	var i Project
	err := row.Scan(
		&i.ID,
		&i.WorkspaceID,
		&i.Name,
		&i.Slug,
		&i.CreatedAt,
	)
	return i, err
}

const getProjectByID = `-- name: GetProjectByID :one
SELECT id, workspace_id, name, slug, created_at FROM projects
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetProjectByID(ctx context.Context, id uuid.UUID) (Project, error) {
	row := q.db.QueryRow(ctx, getProjectByID, id)
	var i Project
	err := row.Scan(
		&i.ID,
		&i.WorkspaceID,
		&i.Name,
		&i.Slug,
		&i.CreatedAt,
	)
	return i, err
}

const getProjectBySlug = `-- name: GetProjectBySlug :one
SELECT id, workspace_id, name, slug, created_at FROM projects
WHERE workspace_id = $1 AND slug = $2 LIMIT 1
`

func (q *Queries) GetProjectBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (Project, error) {
	row := q.db.QueryRow(ctx, getProjectBySlug, workspaceID, slug)
	var i Project
	err := row.Scan(
		&i.ID,
		&i.WorkspaceID,
		&i.Name,
		&i.Slug,
		&i.CreatedAt,
	)
	return i, err
}

const listProjectsByWorkspaceID = `-- name: ListProjectsByWorkspaceID :many
SELECT id, workspace_id, name, slug, created_at FROM projects
WHERE workspace_id = $1
ORDER BY created_at DESC
`

func (q *Queries) ListProjectsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]Project, error) {
	rows, err := q.db.Query(ctx, listProjectsByWorkspaceID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Project
	for rows.Next() {
		var i Project
		if err := rows.Scan(
			&i.ID,
			&i.WorkspaceID,
			&i.Name,
			&i.Slug,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const deleteProject = `-- name: DeleteProject :exec
DELETE FROM projects
WHERE id = $1
`

func (q *Queries) DeleteProject(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteProject, id)
	return err
}

// ============================================================================
// APPLICATIONS
// ============================================================================

const createApplication = `-- name: CreateApplication :one
INSERT INTO applications (
    project_id, name, git_repository_url, git_branch, build_pack,
    dockerfile_path, base_directory, build_command, start_command,
    internal_port, cpu_limit, memory_limit_mb, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, project_id, name, git_repository_url, git_branch, build_pack, dockerfile_path, base_directory, build_command, start_command, internal_port, cpu_limit, memory_limit_mb, status, active_deployment_id, created_at, updated_at
`

type CreateApplicationParams struct {
	ProjectID        uuid.UUID      `json:"project_id"`
	Name             string         `json:"name"`
	GitRepositoryUrl string         `json:"git_repository_url"`
	GitBranch        string         `json:"git_branch"`
	BuildPack        BuildPackType  `json:"build_pack"`
	DockerfilePath   *string        `json:"dockerfile_path"`
	BaseDirectory    *string        `json:"base_directory"`
	BuildCommand     *string        `json:"build_command"`
	StartCommand     *string        `json:"start_command"`
	InternalPort     int32          `json:"internal_port"`
	CpuLimit         pgtype.Numeric `json:"cpu_limit"`
	MemoryLimitMb    *int32         `json:"memory_limit_mb"`
	Status           *string        `json:"status"`
}

func (q *Queries) CreateApplication(ctx context.Context, arg CreateApplicationParams) (Application, error) {
	row := q.db.QueryRow(ctx, createApplication,
		arg.ProjectID,
		arg.Name,
		arg.GitRepositoryUrl,
		arg.GitBranch,
		arg.BuildPack,
		arg.DockerfilePath,
		arg.BaseDirectory,
		arg.BuildCommand,
		arg.StartCommand,
		arg.InternalPort,
		arg.CpuLimit,
		arg.MemoryLimitMb,
		arg.Status,
	)
	var i Application
	err := row.Scan(
		&i.ID,
		&i.ProjectID,
		&i.Name,
		&i.GitRepositoryUrl,
		&i.GitBranch,
		&i.BuildPack,
		&i.DockerfilePath,
		&i.BaseDirectory,
		&i.BuildCommand,
		&i.StartCommand,
		&i.InternalPort,
		&i.CpuLimit,
		&i.MemoryLimitMb,
		&i.Status,
		&i.ActiveDeploymentID,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getApplicationByID = `-- name: GetApplicationByID :one
SELECT id, project_id, name, git_repository_url, git_branch, build_pack, dockerfile_path, base_directory, build_command, start_command, internal_port, cpu_limit, memory_limit_mb, status, active_deployment_id, created_at, updated_at FROM applications
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetApplicationByID(ctx context.Context, id uuid.UUID) (Application, error) {
	row := q.db.QueryRow(ctx, getApplicationByID, id)
	var i Application
	err := row.Scan(
		&i.ID,
		&i.ProjectID,
		&i.Name,
		&i.GitRepositoryUrl,
		&i.GitBranch,
		&i.BuildPack,
		&i.DockerfilePath,
		&i.BaseDirectory,
		&i.BuildCommand,
		&i.StartCommand,
		&i.InternalPort,
		&i.CpuLimit,
		&i.MemoryLimitMb,
		&i.Status,
		&i.ActiveDeploymentID,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const listApplicationsByProjectID = `-- name: ListApplicationsByProjectID :many
SELECT id, project_id, name, git_repository_url, git_branch, build_pack, dockerfile_path, base_directory, build_command, start_command, internal_port, cpu_limit, memory_limit_mb, status, active_deployment_id, created_at, updated_at FROM applications
WHERE project_id = $1
ORDER BY created_at DESC
`

func (q *Queries) ListApplicationsByProjectID(ctx context.Context, projectID uuid.UUID) ([]Application, error) {
	rows, err := q.db.Query(ctx, listApplicationsByProjectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Application
	for rows.Next() {
		var i Application
		if err := rows.Scan(
			&i.ID,
			&i.ProjectID,
			&i.Name,
			&i.GitRepositoryUrl,
			&i.GitBranch,
			&i.BuildPack,
			&i.DockerfilePath,
			&i.BaseDirectory,
			&i.BuildCommand,
			&i.StartCommand,
			&i.InternalPort,
			&i.CpuLimit,
			&i.MemoryLimitMb,
			&i.Status,
			&i.ActiveDeploymentID,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateApplicationStatus = `-- name: UpdateApplicationStatus :one
UPDATE applications
SET status = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, project_id, name, git_repository_url, git_branch, build_pack, dockerfile_path, base_directory, build_command, start_command, internal_port, cpu_limit, memory_limit_mb, status, active_deployment_id, created_at, updated_at
`

func (q *Queries) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status *string) (Application, error) {
	row := q.db.QueryRow(ctx, updateApplicationStatus, id, status)
	var i Application
	err := row.Scan(
		&i.ID,
		&i.ProjectID,
		&i.Name,
		&i.GitRepositoryUrl,
		&i.GitBranch,
		&i.BuildPack,
		&i.DockerfilePath,
		&i.BaseDirectory,
		&i.BuildCommand,
		&i.StartCommand,
		&i.InternalPort,
		&i.CpuLimit,
		&i.MemoryLimitMb,
		&i.Status,
		&i.ActiveDeploymentID,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const updateActiveDeployment = `-- name: UpdateActiveDeployment :one
UPDATE applications
SET active_deployment_id = $2,
    status = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING id, project_id, name, git_repository_url, git_branch, build_pack, dockerfile_path, base_directory, build_command, start_command, internal_port, cpu_limit, memory_limit_mb, status, active_deployment_id, created_at, updated_at
`

func (q *Queries) UpdateActiveDeployment(ctx context.Context, id uuid.UUID, activeDeploymentID *uuid.UUID, status *string) (Application, error) {
	row := q.db.QueryRow(ctx, updateActiveDeployment, id, activeDeploymentID, status)
	var i Application
	err := row.Scan(
		&i.ID,
		&i.ProjectID,
		&i.Name,
		&i.GitRepositoryUrl,
		&i.GitBranch,
		&i.BuildPack,
		&i.DockerfilePath,
		&i.BaseDirectory,
		&i.BuildCommand,
		&i.StartCommand,
		&i.InternalPort,
		&i.CpuLimit,
		&i.MemoryLimitMb,
		&i.Status,
		&i.ActiveDeploymentID,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const updateApplicationResources = `-- name: UpdateApplicationResources :one
UPDATE applications
SET cpu_limit = $2,
    memory_limit_mb = $3,
    internal_port = $4,
    build_command = $5,
    start_command = $6,
    updated_at = NOW()
WHERE id = $1
RETURNING id, project_id, name, git_repository_url, git_branch, build_pack, dockerfile_path, base_directory, build_command, start_command, internal_port, cpu_limit, memory_limit_mb, status, active_deployment_id, created_at, updated_at
`

type UpdateApplicationResourcesParams struct {
	ID            uuid.UUID      `json:"id"`
	CpuLimit      pgtype.Numeric `json:"cpu_limit"`
	MemoryLimitMb *int32         `json:"memory_limit_mb"`
	InternalPort  int32          `json:"internal_port"`
	BuildCommand  *string        `json:"build_command"`
	StartCommand  *string        `json:"start_command"`
}

func (q *Queries) UpdateApplicationResources(ctx context.Context, arg UpdateApplicationResourcesParams) (Application, error) {
	row := q.db.QueryRow(ctx, updateApplicationResources,
		arg.ID,
		arg.CpuLimit,
		arg.MemoryLimitMb,
		arg.InternalPort,
		arg.BuildCommand,
		arg.StartCommand,
	)
	var i Application
	err := row.Scan(
		&i.ID,
		&i.ProjectID,
		&i.Name,
		&i.GitRepositoryUrl,
		&i.GitBranch,
		&i.BuildPack,
		&i.DockerfilePath,
		&i.BaseDirectory,
		&i.BuildCommand,
		&i.StartCommand,
		&i.InternalPort,
		&i.CpuLimit,
		&i.MemoryLimitMb,
		&i.Status,
		&i.ActiveDeploymentID,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const deleteApplication = `-- name: DeleteApplication :exec
DELETE FROM applications
WHERE id = $1
`

func (q *Queries) DeleteApplication(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteApplication, id)
	return err
}

// ============================================================================
// ENVIRONMENT VARIABLES
// ============================================================================

const upsertEnvironmentVariable = `-- name: UpsertEnvironmentVariable :one
INSERT INTO environment_variables (application_id, key, value_encrypted, is_secret)
VALUES ($1, $2, $3, $4)
ON CONFLICT (application_id, key)
DO UPDATE SET
    value_encrypted = EXCLUDED.value_encrypted,
    is_secret = EXCLUDED.is_secret
RETURNING id, application_id, key, value_encrypted, is_secret, created_at
`

type UpsertEnvironmentVariableParams struct {
	ApplicationID  uuid.UUID `json:"application_id"`
	Key            string    `json:"key"`
	ValueEncrypted string    `json:"value_encrypted"`
	IsSecret       *bool     `json:"is_secret"`
}

func (q *Queries) UpsertEnvironmentVariable(ctx context.Context, arg UpsertEnvironmentVariableParams) (EnvironmentVariable, error) {
	row := q.db.QueryRow(ctx, upsertEnvironmentVariable,
		arg.ApplicationID,
		arg.Key,
		arg.ValueEncrypted,
		arg.IsSecret,
	)
	var i EnvironmentVariable
	err := row.Scan(
		&i.ID,
		&i.ApplicationID,
		&i.Key,
		&i.ValueEncrypted,
		&i.IsSecret,
		&i.CreatedAt,
	)
	return i, err
}

const listEnvironmentVariablesByAppID = `-- name: ListEnvironmentVariablesByAppID :many
SELECT id, application_id, key, value_encrypted, is_secret, created_at FROM environment_variables
WHERE application_id = $1
ORDER BY key ASC
`

func (q *Queries) ListEnvironmentVariablesByAppID(ctx context.Context, applicationID uuid.UUID) ([]EnvironmentVariable, error) {
	rows, err := q.db.Query(ctx, listEnvironmentVariablesByAppID, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []EnvironmentVariable
	for rows.Next() {
		var i EnvironmentVariable
		if err := rows.Scan(
			&i.ID,
			&i.ApplicationID,
			&i.Key,
			&i.ValueEncrypted,
			&i.IsSecret,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const deleteEnvironmentVariable = `-- name: DeleteEnvironmentVariable :exec
DELETE FROM environment_variables
WHERE application_id = $1 AND key = $2
`

func (q *Queries) DeleteEnvironmentVariable(ctx context.Context, applicationID uuid.UUID, key string) error {
	_, err := q.db.Exec(ctx, deleteEnvironmentVariable, applicationID, key)
	return err
}

// ============================================================================
// DEPLOYMENTS
// ============================================================================

const createDeployment = `-- name: CreateDeployment :one
INSERT INTO deployments (
    application_id, triggered_by, trigger_source, status
)
VALUES ($1, $2, $3, $4)
RETURNING id, application_id, triggered_by, trigger_source, status, commit_hash, commit_message, commit_author, docker_image_tag, container_id, started_at, finished_at, duration_ms, created_at
`

type CreateDeploymentParams struct {
	ApplicationID uuid.UUID        `json:"application_id"`
	TriggeredBy   *uuid.UUID       `json:"triggered_by"`
	TriggerSource TriggerSource    `json:"trigger_source"`
	Status        DeploymentStatus `json:"status"`
}

func (q *Queries) CreateDeployment(ctx context.Context, arg CreateDeploymentParams) (Deployment, error) {
	row := q.db.QueryRow(ctx, createDeployment,
		arg.ApplicationID,
		arg.TriggeredBy,
		arg.TriggerSource,
		arg.Status,
	)
	var i Deployment
	err := row.Scan(
		&i.ID,
		&i.ApplicationID,
		&i.TriggeredBy,
		&i.TriggerSource,
		&i.Status,
		&i.CommitHash,
		&i.CommitMessage,
		&i.CommitAuthor,
		&i.DockerImageTag,
		&i.ContainerID,
		&i.StartedAt,
		&i.FinishedAt,
		&i.DurationMs,
		&i.CreatedAt,
	)
	return i, err
}

const getDeploymentByID = `-- name: GetDeploymentByID :one
SELECT id, application_id, triggered_by, trigger_source, status, commit_hash, commit_message, commit_author, docker_image_tag, container_id, started_at, finished_at, duration_ms, created_at FROM deployments
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetDeploymentByID(ctx context.Context, id uuid.UUID) (Deployment, error) {
	row := q.db.QueryRow(ctx, getDeploymentByID, id)
	var i Deployment
	err := row.Scan(
		&i.ID,
		&i.ApplicationID,
		&i.TriggeredBy,
		&i.TriggerSource,
		&i.Status,
		&i.CommitHash,
		&i.CommitMessage,
		&i.CommitAuthor,
		&i.DockerImageTag,
		&i.ContainerID,
		&i.StartedAt,
		&i.FinishedAt,
		&i.DurationMs,
		&i.CreatedAt,
	)
	return i, err
}

const listDeploymentsByAppID = `-- name: ListDeploymentsByAppID :many
SELECT id, application_id, triggered_by, trigger_source, status, commit_hash, commit_message, commit_author, docker_image_tag, container_id, started_at, finished_at, duration_ms, created_at FROM deployments
WHERE application_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

func (q *Queries) ListDeploymentsByAppID(ctx context.Context, applicationID uuid.UUID, limit, offset int32) ([]Deployment, error) {
	rows, err := q.db.Query(ctx, listDeploymentsByAppID, applicationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Deployment
	for rows.Next() {
		var i Deployment
		if err := rows.Scan(
			&i.ID,
			&i.ApplicationID,
			&i.TriggeredBy,
			&i.TriggerSource,
			&i.Status,
			&i.CommitHash,
			&i.CommitMessage,
			&i.CommitAuthor,
			&i.DockerImageTag,
			&i.ContainerID,
			&i.StartedAt,
			&i.FinishedAt,
			&i.DurationMs,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateDeploymentStatus = `-- name: UpdateDeploymentStatus :one
UPDATE deployments
SET status = $2,
    started_at = COALESCE(started_at, CASE WHEN $2 = 'cloning'::deployment_status THEN NOW() ELSE started_at END)
WHERE id = $1
RETURNING id, application_id, triggered_by, trigger_source, status, commit_hash, commit_message, commit_author, docker_image_tag, container_id, started_at, finished_at, duration_ms, created_at
`

func (q *Queries) UpdateDeploymentStatus(ctx context.Context, id uuid.UUID, status DeploymentStatus) (Deployment, error) {
	row := q.db.QueryRow(ctx, updateDeploymentStatus, id, status)
	var i Deployment
	err := row.Scan(
		&i.ID,
		&i.ApplicationID,
		&i.TriggeredBy,
		&i.TriggerSource,
		&i.Status,
		&i.CommitHash,
		&i.CommitMessage,
		&i.CommitAuthor,
		&i.DockerImageTag,
		&i.ContainerID,
		&i.StartedAt,
		&i.FinishedAt,
		&i.DurationMs,
		&i.CreatedAt,
	)
	return i, err
}

const updateDeploymentCommitInfo = `-- name: UpdateDeploymentCommitInfo :one
UPDATE deployments
SET commit_hash = $2,
    commit_message = $3,
    commit_author = $4
WHERE id = $1
RETURNING id, application_id, triggered_by, trigger_source, status, commit_hash, commit_message, commit_author, docker_image_tag, container_id, started_at, finished_at, duration_ms, created_at
`

func (q *Queries) UpdateDeploymentCommitInfo(ctx context.Context, id uuid.UUID, hash, message, author *string) (Deployment, error) {
	row := q.db.QueryRow(ctx, updateDeploymentCommitInfo, id, hash, message, author)
	var i Deployment
	err := row.Scan(
		&i.ID,
		&i.ApplicationID,
		&i.TriggeredBy,
		&i.TriggerSource,
		&i.Status,
		&i.CommitHash,
		&i.CommitMessage,
		&i.CommitAuthor,
		&i.DockerImageTag,
		&i.ContainerID,
		&i.StartedAt,
		&i.FinishedAt,
		&i.DurationMs,
		&i.CreatedAt,
	)
	return i, err
}

const updateDeploymentContainer = `-- name: UpdateDeploymentContainer :one
UPDATE deployments
SET docker_image_tag = $2,
    container_id = $3
WHERE id = $1
RETURNING id, application_id, triggered_by, trigger_source, status, commit_hash, commit_message, commit_author, docker_image_tag, container_id, started_at, finished_at, duration_ms, created_at
`

func (q *Queries) UpdateDeploymentContainer(ctx context.Context, id uuid.UUID, imageTag, containerID *string) (Deployment, error) {
	row := q.db.QueryRow(ctx, updateDeploymentContainer, id, imageTag, containerID)
	var i Deployment
	err := row.Scan(
		&i.ID,
		&i.ApplicationID,
		&i.TriggeredBy,
		&i.TriggerSource,
		&i.Status,
		&i.CommitHash,
		&i.CommitMessage,
		&i.CommitAuthor,
		&i.DockerImageTag,
		&i.ContainerID,
		&i.StartedAt,
		&i.FinishedAt,
		&i.DurationMs,
		&i.CreatedAt,
	)
	return i, err
}

const finishDeployment = `-- name: FinishDeployment :one
UPDATE deployments
SET status = $2,
    finished_at = NOW(),
    duration_ms = EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000
WHERE id = $1
RETURNING id, application_id, triggered_by, trigger_source, status, commit_hash, commit_message, commit_author, docker_image_tag, container_id, started_at, finished_at, duration_ms, created_at
`

func (q *Queries) FinishDeployment(ctx context.Context, id uuid.UUID, status DeploymentStatus) (Deployment, error) {
	row := q.db.QueryRow(ctx, finishDeployment, id, status)
	var i Deployment
	err := row.Scan(
		&i.ID,
		&i.ApplicationID,
		&i.TriggeredBy,
		&i.TriggerSource,
		&i.Status,
		&i.CommitHash,
		&i.CommitMessage,
		&i.CommitAuthor,
		&i.DockerImageTag,
		&i.ContainerID,
		&i.StartedAt,
		&i.FinishedAt,
		&i.DurationMs,
		&i.CreatedAt,
	)
	return i, err
}

// ============================================================================
// DEPLOYMENT LOGS
// ============================================================================

const insertDeploymentLog = `-- name: InsertDeploymentLog :one
INSERT INTO deployment_logs (deployment_id, stream, message)
VALUES ($1, $2, $3)
RETURNING id, deployment_id, stream, message, created_at
`

func (q *Queries) InsertDeploymentLog(ctx context.Context, deploymentID uuid.UUID, stream *string, message string) (DeploymentLog, error) {
	row := q.db.QueryRow(ctx, insertDeploymentLog, deploymentID, stream, message)
	var i DeploymentLog
	err := row.Scan(
		&i.ID,
		&i.DeploymentID,
		&i.Stream,
		&i.Message,
		&i.CreatedAt,
	)
	return i, err
}

const listDeploymentLogsByDeploymentID = `-- name: ListDeploymentLogsByDeploymentID :many
SELECT id, deployment_id, stream, message, created_at FROM deployment_logs
WHERE deployment_id = $1
ORDER BY id ASC
`

func (q *Queries) ListDeploymentLogsByDeploymentID(ctx context.Context, deploymentID uuid.UUID) ([]DeploymentLog, error) {
	rows, err := q.db.Query(ctx, listDeploymentLogsByDeploymentID, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DeploymentLog
	for rows.Next() {
		var i DeploymentLog
		if err := rows.Scan(
			&i.ID,
			&i.DeploymentID,
			&i.Stream,
			&i.Message,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// ============================================================================
// DOMAINS
// ============================================================================

const createDomain = `-- name: CreateDomain :one
INSERT INTO domains (application_id, domain, is_subdomain, ssl_status)
VALUES ($1, $2, $3, $4)
RETURNING id, application_id, domain, is_subdomain, ssl_status, created_at
`

func (q *Queries) CreateDomain(ctx context.Context, appID uuid.UUID, domain string, isSubdomain *bool, sslStatus *string) (Domain, error) {
	row := q.db.QueryRow(ctx, createDomain, appID, domain, isSubdomain, sslStatus)
	var i Domain
	err := row.Scan(
		&i.ID,
		&i.ApplicationID,
		&i.Domain,
		&i.IsSubdomain,
		&i.SslStatus,
		&i.CreatedAt,
	)
	return i, err
}

const getDomainByName = `-- name: GetDomainByName :one
SELECT id, application_id, domain, is_subdomain, ssl_status, created_at FROM domains
WHERE domain = $1 LIMIT 1
`

func (q *Queries) GetDomainByName(ctx context.Context, domain string) (Domain, error) {
	row := q.db.QueryRow(ctx, getDomainByName, domain)
	var i Domain
	err := row.Scan(
		&i.ID,
		&i.ApplicationID,
		&i.Domain,
		&i.IsSubdomain,
		&i.SslStatus,
		&i.CreatedAt,
	)
	return i, err
}

const listDomainsByAppID = `-- name: ListDomainsByAppID :many
SELECT id, application_id, domain, is_subdomain, ssl_status, created_at FROM domains
WHERE application_id = $1
ORDER BY created_at ASC
`

func (q *Queries) ListDomainsByAppID(ctx context.Context, appID uuid.UUID) ([]Domain, error) {
	rows, err := q.db.Query(ctx, listDomainsByAppID, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Domain
	for rows.Next() {
		var i Domain
		if err := rows.Scan(
			&i.ID,
			&i.ApplicationID,
			&i.Domain,
			&i.IsSubdomain,
			&i.SslStatus,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const deleteDomain = `-- name: DeleteDomain :exec
DELETE FROM domains
WHERE id = $1
`

func (q *Queries) DeleteDomain(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteDomain, id)
	return err
}
