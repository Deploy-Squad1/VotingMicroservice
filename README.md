# Voting Microservice

A Go microservice for creating and voting on user moderation/upgrade polls.

It currently supports:
- Creating upgrade polls (`Bronze -> Silver -> Gold` flow rules)
- Creating kick polls (Inquisitor only)
- Voting on active polls (`Silver` and `Gold` can vote)
- Processing expired polls and applying outcomes


## Prerequisites

- Go installed (compatible with the version in `go.mod`)
- PostgreSQL running
- A database with required external tables already present:
  - `app_user`
  - `app_user_groups`
  - `map_artifacts`

The included migration creates only:
- `polls`
- `polls_votes`

## Environment Variables

Create a `.env` file in project root:

```env
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_HOST=your_db_host
DB_PORT=your_db_port
DB_NAME=your_db_name
JWT_SECRET=your_jwt_secret
```


## Run the Service

Migrations are executed automatically on startup by `runMigrations(...)` in `cmd/api/main.go`.

```
go mod tidy
go run ./cmd/api/main.go
```

Default server address:
- `http://localhost:8085`

## Connect a Cron Docker Container

If your scheduler runs in Docker, it should call the API using the **container service name** (not `localhost`).

- Endpoint to trigger: `POST /api/internal/polls/process`
- API container port in code: `8085`
- Example internal URL from scheduler: `http://here_is_container_service_name:8085/api/internal/polls/process`
- Recommended: make the cron which will call this endpoint run every 5-10 minutes to ensure timely processing of expired polls.
- Example cron command using `curl`: ```* * * * * curl -X POST http://here_is_container_service_name:8085/api/internal/polls/process >> /proc/1/fd/1 2>&1```

## API Endpoints

### 1) Create Upgrade Poll

- **Method/Path:** `POST /api/polls/upgrade`
- **Auth:** required (`access_token` cookie), any authenticated role
- **Behavior:**
  - Rejects if user is already `Gold`
  - Requires minimum time since join:
    - group `1`: 24h
    - group `2`: 72h
  - Rejects if user already has active upgrade poll

### 2) Create Kick Poll

- **Method/Path:** `POST /api/polls/kick/user/{target_id}`
- **Auth:** required (`access_token` cookie), any authenticated role
- **Extra Rule:** requester must be inquisitor (`is_inquisitor` from DB)
- **Behavior:**
  - Rejects self-kick
  - Rejects if target already has active kick poll

### 3) Vote On Poll

- **Method/Path:** `POST /api/polls/{poll_id}/vote`
- **Auth:** required (`access_token` cookie), allowed roles: `Silver`, `Gold`
- **Behavior:**
  - Poll must exist and be `active`
  - One vote per voter per poll (DB unique constraint)

### 4) Process Expired Polls

- **Method/Path:** `POST /api/internal/polls/process`
- **Auth:** currently no middleware in `cmd/api/main.go`
- **Behavior:**
  - Loads expired active polls
  - Counts votes
  - Marks as `passed` when `votes_for > votes_against`, else `failed`
  - Applies effect:
    - `kick`: deletes user-related records via `ExecuteKick`
    - `upgrade`: increments `group_id` via `ExecuteUpgrade`

### 5) Get Active Polls

- **Method/Path:** `GET /api/polls/active`
- **Auth:** required (`access_token` cookie), any authenticated role
- **Behavior:**
  - Returns list of active polls with details (type, target user, time remaining, etc.)
  - Useful for frontend display of ongoing polls

### 6) Database Delete

- **Method/Path:** `DELETE /api/internal/database/delete`
- **Auth:** requires authentication of special user `access_token` coockie
- **Behavior:**
- Deletes all records from `polls` and `polls_votes` tables