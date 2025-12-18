-- Drop repository_owners table

DROP INDEX IF EXISTS idx_repository_owners_team_slug;
DROP INDEX IF EXISTS idx_repository_owners_repo;
DROP TABLE IF EXISTS repository_owners;
