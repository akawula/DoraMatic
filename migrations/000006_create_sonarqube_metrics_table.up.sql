-- migrations/000006_create_sonarqube_metrics_table.up.sql

CREATE TABLE IF NOT EXISTS sonarqube_projects (
    project_key VARCHAR(500) PRIMARY KEY,
    project_name VARCHAR(500) NOT NULL,
    last_analyzed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sonarqube_metrics (
    id SERIAL PRIMARY KEY,
    project_key VARCHAR(500) NOT NULL REFERENCES sonarqube_projects(project_key) ON DELETE CASCADE,
    metric_key VARCHAR(100) NOT NULL,
    metric_value DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_key, metric_key, recorded_at)
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_sonarqube_metrics_project ON sonarqube_metrics (project_key);
CREATE INDEX IF NOT EXISTS idx_sonarqube_metrics_recorded ON sonarqube_metrics (recorded_at);
CREATE INDEX IF NOT EXISTS idx_sonarqube_metrics_key ON sonarqube_metrics (metric_key);
CREATE INDEX IF NOT EXISTS idx_sonarqube_projects_updated ON sonarqube_projects (updated_at);
