-- ============================================================================
-- USERS & AUTH
-- ============================================================================

-- name: CreateUser :one
INSERT INTO users (email, password_hash, name, avatar_url, system_role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE($2, name),
    avatar_url = COALESCE($3, avatar_url),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- ============================================================================
-- WORKSPACES & MEMBERS
-- ============================================================================

-- name: CreateWorkspace :one
INSERT INTO workspaces (name, slug, owner_id, max_cpus, max_memory_mb)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetWorkspaceByID :one
SELECT * FROM workspaces
WHERE id = $1 LIMIT 1;

-- name: GetWorkspaceBySlug :one
SELECT * FROM workspaces
WHERE slug = $1 LIMIT 1;

-- name: ListWorkspacesForUser :many
SELECT w.*, wm.role as member_role
FROM workspaces w
INNER JOIN workspace_members wm ON w.id = wm.workspace_id
WHERE wm.user_id = $1
ORDER BY w.created_at DESC;

-- name: AddWorkspaceMember :one
INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWorkspaceMemberRole :one
SELECT role FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2 LIMIT 1;

-- ============================================================================
-- PROJECTS
-- ============================================================================

-- name: CreateProject :one
INSERT INTO projects (workspace_id, name, slug)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetProjectByID :one
SELECT * FROM projects
WHERE id = $1 LIMIT 1;

-- name: GetProjectBySlug :one
SELECT * FROM projects
WHERE workspace_id = $1 AND slug = $2 LIMIT 1;

-- name: ListProjectsByWorkspaceID :many
SELECT * FROM projects
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: DeleteProject :exec
DELETE FROM projects
WHERE id = $1;

-- ============================================================================
-- APPLICATIONS
-- ============================================================================

-- name: CreateApplication :one
INSERT INTO applications (
    project_id, name, git_repository_url, git_branch, build_pack,
    dockerfile_path, base_directory, build_command, start_command,
    internal_port, cpu_limit, memory_limit_mb, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetApplicationByID :one
SELECT * FROM applications
WHERE id = $1 LIMIT 1;

-- name: ListApplicationsByProjectID :many
SELECT * FROM applications
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: UpdateApplicationStatus :one
UPDATE applications
SET status = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateActiveDeployment :one
UPDATE applications
SET active_deployment_id = $2,
    status = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateApplicationResources :one
UPDATE applications
SET cpu_limit = $2,
    memory_limit_mb = $3,
    internal_port = $4,
    build_command = $5,
    start_command = $6,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteApplication :exec
DELETE FROM applications
WHERE id = $1;

-- ============================================================================
-- ENVIRONMENT VARIABLES (AES-256 Encrypted)
-- ============================================================================

-- name: UpsertEnvironmentVariable :one
INSERT INTO environment_variables (application_id, key, value_encrypted, is_secret)
VALUES ($1, $2, $3, $4)
ON CONFLICT (application_id, key)
DO UPDATE SET
    value_encrypted = EXCLUDED.value_encrypted,
    is_secret = EXCLUDED.is_secret
RETURNING *;

-- name: ListEnvironmentVariablesByAppID :many
SELECT * FROM environment_variables
WHERE application_id = $1
ORDER BY key ASC;

-- name: DeleteEnvironmentVariable :exec
DELETE FROM environment_variables
WHERE application_id = $1 AND key = $2;

-- ============================================================================
-- DEPLOYMENTS
-- ============================================================================

-- name: CreateDeployment :one
INSERT INTO deployments (
    application_id, triggered_by, trigger_source, status
)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDeploymentByID :one
SELECT * FROM deployments
WHERE id = $1 LIMIT 1;

-- name: ListDeploymentsByAppID :many
SELECT * FROM deployments
WHERE application_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateDeploymentStatus :one
UPDATE deployments
SET status = $2,
    started_at = COALESCE(started_at, CASE WHEN $2 = 'cloning'::deployment_status THEN NOW() ELSE started_at END)
WHERE id = $1
RETURNING *;

-- name: UpdateDeploymentCommitInfo :one
UPDATE deployments
SET commit_hash = $2,
    commit_message = $3,
    commit_author = $4
WHERE id = $1
RETURNING *;

-- name: UpdateDeploymentContainer :one
UPDATE deployments
SET docker_image_tag = $2,
    container_id = $3
WHERE id = $1
RETURNING *;

-- name: FinishDeployment :one
UPDATE deployments
SET status = $2,
    finished_at = NOW(),
    duration_ms = EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000
WHERE id = $1
RETURNING *;

-- ============================================================================
-- DEPLOYMENT LOGS
-- ============================================================================

-- name: InsertDeploymentLog :one
INSERT INTO deployment_logs (deployment_id, stream, message)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListDeploymentLogsByDeploymentID :many
SELECT * FROM deployment_logs
WHERE deployment_id = $1
ORDER BY id ASC;

-- ============================================================================
-- DOMAINS
-- ============================================================================

-- name: CreateDomain :one
INSERT INTO domains (application_id, domain, is_subdomain, ssl_status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDomainByName :one
SELECT * FROM domains
WHERE domain = $1 LIMIT 1;

-- name: ListDomainsByAppID :many
SELECT * FROM domains
WHERE application_id = $1
ORDER BY created_at ASC;

-- name: DeleteDomain :exec
DELETE FROM domains
WHERE id = $1;
