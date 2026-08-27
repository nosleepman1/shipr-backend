DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS deployment_logs;
ALTER TABLE IF EXISTS applications DROP CONSTRAINT IF EXISTS fk_active_deployment;
DROP TABLE IF EXISTS deployments;
DROP TABLE IF EXISTS environment_variables;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS trigger_source;
DROP TYPE IF EXISTS deployment_status;
DROP TYPE IF EXISTS build_pack_type;
DROP TYPE IF EXISTS workspace_role;
DROP TYPE IF EXISTS auth_provider;
DROP TYPE IF EXISTS user_role;
