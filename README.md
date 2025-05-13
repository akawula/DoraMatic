# DoraMatic

## Project Overview

The DoraMatic is designed to provide insights into software development processes by collecting and analyzing data from various sources like GitHub and Jira. It offers a web interface to visualize metrics, team statistics, and pull request lead times. The tool also includes capabilities for Slack notifications and cron-based data collection.

## Features

*   **Pull Request Analysis:** Fetches and analyzes pull request data from GitHub, including lead times.
*   **Jira Integration:** Tracks and links Jira references.
*   **Team Statistics:** Calculates and displays statistics for development teams.
*   **Slack Notifications:** Provides updates and alerts via Slack.
*   **Cron Jobs:** Automates data collection and processing tasks.
*   **Web Interface:** A React-based frontend for visualizing data and interacting with the tool.
*   **API Endpoints:** Exposes a Go-based API for data retrieval and operations.

## Prerequisites

Before you begin, ensure you have the following installed:

*   **Go:** (Specify version, e.g., 1.18 or later) - For the backend server.
*   **Node.js and npm:** (Specify versions, e.g., Node.js 16.x, npm 8.x or later) - For the frontend application.
*   **PostgreSQL:** (Specify version, e.g., 13 or later) - As the database.
*   **Docker:** (Optional, but recommended for cron jobs and deployment)
*   **Make:** For using Makefile commands.

## Setup and Installation

### 1. Clone the Repository

```bash
git clone <your-repository-url>
cd doramatic
```

### 2. Backend Setup (Go)

Navigate to the project root directory.

*   **Install Dependencies:**
    ```bash
    go mod tidy
    ```

### 3. Frontend Setup (React)

Navigate to the `frontend` directory:

```bash
cd frontend
```

*   **Install Dependencies:**
    ```bash
    npm install
    ```
Back to the root directory:
```bash
cd ..
```

### 4. Database Setup (PostgreSQL)

*   Ensure your PostgreSQL server is running.
*   Create a database for the application (e.g., `doramatic_db`).
*   Configure the database connection string (see Configuration section).
*   **Run Migrations:**
    The project uses SQL migrations located in the `migrations/` directory. You'll need a migration tool compatible with Go (e.g., `migrate`, `sql-migrate`, or use the project's built-in mechanism if available). Assuming a `make` command exists for this:
    ```bash
    make migrate-up
    ```
    If a direct make command isn't available, you'll need to manually apply the `.up.sql` files in order using a PostgreSQL client like `psql`.

## Configuration

The application likely requires environment variables for its configuration. Create a `.env` file in the root directory or set these variables in your environment:

```env
# Server Configuration
API_PORT=8080

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=doramatic_db
DB_SSL_MODE=disable # or 'require' depending on your setup

# GitHub Integration
GITHUB_TOKEN=your_github_personal_access_token
GITHUB_ORG=your_github_organization_name

# Jira Integration (if applicable, details might vary)
JIRA_URL=https_your_org_atlassian_net
JIRA_USERNAME=your_jira_username
JIRA_API_TOKEN=your_jira_api_token

# Slack Integration
SLACK_BOT_TOKEN=your_slack_bot_token
SLACK_CHANNEL_ID=your_slack_channel_id

# Other configurations
# Example: CRON_SCHEDULE="0 0 * * *"
```
**Note:** The exact environment variable names might differ. Check the codebase (e.g., `config.go`, `main.go`, or how `os.Getenv` is used) for the precise names.

## Running the Application

### 1. Backend Server

From the project root directory:

*   **Using `air` (for live reloading, if configured in `.air.toml`):**
    ```bash
    air
    ```
*   **Or, build and run manually:**
    ```bash
    go build -o doramatic_server ./cmd/server
    ./doramatic_server
    ```
    Alternatively, use a make command if available:
    ```bash
    make run-server
    ```

The backend server will typically start on `http://localhost:8080` (or the port specified in `API_PORT`).

### 2. Frontend Application

Navigate to the `frontend` directory:

```bash
cd frontend
```

*   **Start the development server:**
    ```bash
    npm start
    ```
The frontend will typically be accessible at `http://localhost:3000`.

### 3. Cron Job

The `Dockerfile.cron` suggests a cron job component.

*   **Building the Docker image for cron:**
    ```bash
    docker build -t doramatic-cron -f Dockerfile.cron .
    ```
*   **Running the cron job container:**
    You'll need to pass the necessary environment variables to the Docker container.
    ```bash
    docker run -d --env-file .env doramatic-cron
    ```
    (Ensure your `.env` file is correctly formatted for Docker's `--env-file` option or pass variables individually with `-e`).

## Usage

*   **Web Interface:** Access the frontend URL (e.g., `http://localhost:3000`) in your browser to interact with the application, view dashboards, and analyze metrics.
*   **API Endpoints:** The backend exposes API endpoints (defined in `swagger.yaml` and implemented in `cmd/server/handlers/`). You can use tools like `curl` or Postman to interact with these endpoints. Refer to `swagger.yaml` for detailed API documentation.

## Deployment

*   The backend server can be built into a binary and deployed.
*   The frontend application can be built into static assets for serving:
    ```bash
    cd frontend
    npm run build
    cd ..
    ```
*   Docker can be used for containerizing both the backend server and the cron jobs for easier deployment and scaling. Consider creating a `Dockerfile` for the main server application as well.

---

This README provides a general outline. You may need to adjust specific commands, version numbers, and configuration details based on the exact implementation of the "DoraMatic" tool.
