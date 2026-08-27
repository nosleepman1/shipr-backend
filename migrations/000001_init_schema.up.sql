-- Enable pgcrypto for UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Custom ENUM Types
DO $$ BEGIN
    CREATE TYPE user_role AS ENUM ('superadmin', 'user');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE auth_provider AS ENUM ('local', 'github', 'gitlab', 'google');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE workspace_role AS ENUM ('owner', 'admin', 'developer', 'viewer');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE build_pack_type AS ENUM ('nixpacks', 'dockerfile', 'static');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE deployment_status AS ENUM ('queued', 'cloning', 'building', 'deploying', 'running', 'failed', 'cancelled');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE trigger_source AS ENUM ('manual', 'webhook', 'rollback', 'api');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- 1. Users Table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    email_verified_at TIMESTAMPTZ,
    password_hash VARCHAR(255),
    name VARCHAR(150),
    avatar_url TEXT,
    system_role user_role DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. OAuth Accounts Table
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider auth_provider NOT NULL,
    provider_account_id VARCHAR(255) NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_provider_account UNIQUE(provider, provider_account_id)
);

-- 3. Workspaces Table
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(120) NOT NULL UNIQUE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    max_cpus NUMERIC(4,2) DEFAULT 2.0,
    max_memory_mb INT DEFAULT 2048,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Workspace Members Table
CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role workspace_role NOT NULL DEFAULT 'developer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id)
);

-- 5. Projects Table
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(120) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_workspace_project_slug UNIQUE(workspace_id, slug)
);

-- 6. Applications Table
CREATE TABLE IF NOT EXISTS applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    git_repository_url VARCHAR(500) NOT NULL,
    git_branch VARCHAR(100) NOT NULL DEFAULT 'main',
    build_pack build_pack_type NOT NULL DEFAULT 'nixpacks',
    dockerfile_path VARCHAR(255) DEFAULT 'Dockerfile',
    base_directory VARCHAR(255) DEFAULT '/',
    build_command TEXT,
    start_command TEXT,
    internal_port INT NOT NULL DEFAULT 3000,
    cpu_limit NUMERIC(4,2) DEFAULT 0.5,
    memory_limit_mb INT DEFAULT 512,
    status VARCHAR(50) DEFAULT 'idle',
    active_deployment_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 7. Environment Variables Table (AES-256 Encrypted)
CREATE TABLE IF NOT EXISTS environment_variables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value_encrypted TEXT NOT NULL,
    is_secret BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_app_env_key UNIQUE(application_id, key)
);

-- 8. Deployments Table
CREATE TABLE IF NOT EXISTS deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    triggered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    trigger_source trigger_source NOT NULL DEFAULT 'manual',
    status deployment_status NOT NULL DEFAULT 'queued',
    commit_hash VARCHAR(40),
    commit_message TEXT,
    commit_author VARCHAR(150),
    docker_image_tag VARCHAR(255),
    container_id VARCHAR(128),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    duration_ms INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Circular foreign key on applications for active_deployment_id
DO $$ BEGIN
    ALTER TABLE applications 
    ADD CONSTRAINT fk_active_deployment 
    FOREIGN KEY (active_deployment_id) REFERENCES deployments(id) ON DELETE SET NULL;
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- 9. Deployment Logs Table
CREATE TABLE IF NOT EXISTS deployment_logs (
    id BIGSERIAL PRIMARY KEY,
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    stream VARCHAR(10) DEFAULT 'stdout',
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 10. Domains Table
CREATE TABLE IF NOT EXISTS domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL UNIQUE,
    is_subdomain BOOLEAN DEFAULT FALSE,
    ssl_status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for high performance
CREATE INDEX IF NOT EXISTS idx_projects_workspace_id ON projects(workspace_id);
CREATE INDEX IF NOT EXISTS idx_applications_project_id ON applications(project_id);
CREATE INDEX IF NOT EXISTS idx_deployments_application_id ON deployments(application_id);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_id ON deployment_logs(deployment_id);
CREATE INDEX IF NOT EXISTS idx_domains_domain ON domains(domain);
CREATE INDEX IF NOT EXISTS idx_domains_application_id ON domains(application_id);
