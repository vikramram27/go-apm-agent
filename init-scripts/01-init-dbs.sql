-- Create separate databases for notes-api and notes-api-gin
CREATE DATABASE notes_api;
CREATE DATABASE notes_api_gin;

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE notes_api TO postgres;
GRANT ALL PRIVILEGES ON DATABASE notes_api_gin TO postgres;
