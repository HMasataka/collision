# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

All commands use `task` (Taskfile.yml):

### Quick Start
- `task dev` - Build and run the collision server
- `task run-client` - Run simpleticket client with `go run`
- `task check` - Run formatting, vet, and tests (use before committing)

### Building
- `task build` - Build collision server and simpleticket client binaries to `./bin/`
- `task build-all` - Build binaries for Linux, macOS (amd64/arm64), and Windows

### Running
- `task run` - Run collision server directly with `go run`
- `task run-client` - Run simpleticket client with `go run`

### Code Quality
- `task fmt` - Format code with `go fmt ./...`
- `task vet` - Run `go vet ./...`
- `task test` - Run all tests with `go test -v ./...`

### Maintenance
- `task proto` - Generate Go files from Protocol Buffer definitions (required after any `.proto` changes)
- `task mod-tidy` - Tidy dependencies with `go mod tidy`
- `task clean` - Remove build artifacts

## Project Architecture

**Collision** is a real-time 1v1 matchmaking server built with gRPC. The architecture follows Clean Architecture principles with clear separation of concerns:

### Core Flow

1. **Clients** submit tickets via `CreateTicket` gRPC call
2. **Match Loop** (runs every 1 second in `cmd/collision/main.go:95`) fetches active tickets and executes matching logic
3. **Matching** groups tickets according to `MatchProfile` and pools, then calls the registered `MatchFunction`
4. **Assignment** generates server assignments for matched tickets via the `Assigner` function
5. **Clients** watch for assignments via `WatchAssignments` streaming API

### Layer Breakdown

**handler/** - gRPC endpoint handlers

- Maps protobuf messages to domain entities
- Delegates to usecases
- `frontend.go` implements `FrontendService` (CreateTicket, WatchAssignments, etc.)

**usecase/** - Business logic orchestration

- `ticket.go` - Creates tickets with unique IDs using `xid`, stores in Redis with 10-minute TTL
- `match.go` - Main matching engine: fetches active tickets, calls match functions per profile, handles assignment
- `assign.go` - Watches ticket assignments and notifies clients via Redis pub/sub

**domain/entity/** - Domain models

- `ticket.go` - Core `Ticket` struct with SearchFields and Assignment
- `match.go` - `Match`, `MatchProfile`, `Pool`, `MatchFunction` (extensible matching logic)
- `assign.go` - `Assignment`, `AssignmentGroup`, `Assigner` (extensible assignment logic)

**infrastructure/**

- `redis.go` - Redis client and locker initialization (connects to `127.0.0.1:6379`)
- `persistence/ticket.go` - Redis-backed ticket repository with locking

**domain/repository/** - Repository interface

- `ticket.go` - `TicketRepository` interface defining data access operations

### Key Extension Points

**Match Function** (`cmd/collision/main.go:52-54`) - Register custom match functions in the `matchFunctions` map:

- Implement `entity.MatchFunction` interface (input: tickets grouped by pool, output: `[]entity.Match`)
- Default: `Simple1vs1MatchFunction` pairs tickets in groups of 2
- Multiple match profiles can have different matching algorithms

**Assigner Function** (`cmd/collision/main.go:56`) - Replace `NewRandomAssigner()` with custom implementation:

- Implement `entity.Assigner` interface (input: `[]entity.Match`, output: `*entity.AssignmentGroup`)
- Default: `RandomAssigner` generates random server names via `hri.Random()`
- Used for determining server assignment after tickets are matched

### Dependencies

Key external libraries in `go.mod`:

- `google.golang.org/grpc` - gRPC framework
- `github.com/redis/rueidis` - Redis client with locking support
- `github.com/bojand/hri` - Generate human-readable random names
- `github.com/rs/xid` - Generate globally unique IDs
- `google.golang.org/protobuf` - Protocol Buffers

### Protocol Buffers

Proto definitions in `api/`:

- `frontend.proto` - FrontendService API (CreateTicket, WatchAssignments, etc.)
- `messages.proto` - Core message types (Ticket, Assignment, etc.)

Regenerate with `task proto` after changes.

## Local Development Setup

### Prerequisites
- Go 1.24+
- Redis (required for server operation)
- Protocol Buffers compiler (`protoc`) for generating code from `.proto` files

### Starting Redis

Use Docker Compose (recommended):
```bash
docker-compose up -d redis
```

Or run Redis directly:
```bash
redis-server  # if installed locally
# or
docker run -d -p 6379:6379 redis:alpine
```

## Development Notes

- **Redis requirement**: Server requires Redis running on `127.0.0.1:6379` (hardcoded in `infrastructure/redis.go:13`)
- **Default port**: gRPC server listens on `127.0.0.1:31080` (see `cmd/collision/main.go:20`)
- **Match interval**: Matching logic executes every 1 second (see `cmd/collision/main.go:94`)
- **Ticket TTL**: Newly created tickets expire in 10 minutes (see `usecase/ticket.go:36`)
- **Lock TTL**: Redis locks have 1-second timeout (see `infrastructure/redis.go:10`)

## Key Data Flow Concepts

### Ticket Lifecycle
1. **Creation** (`handler/frontend.go`): Client calls `CreateTicket`, ticket stored in Redis with 10-minute expiration
2. **Matching** (`usecase/match.go`): Every 1 second, the match loop fetches all active tickets
3. **Assignment** (`usecase/assign.go`): Matched tickets receive server assignment via Redis pub/sub
4. **Cleanup**: Clients delete tickets manually or they expire after 10 minutes

### Matching Profiles
- Tickets are grouped by `MatchProfile` name (e.g., "simple-1vs1")
- Each profile can have its own matching algorithm
- Profiles are registered in `cmd/collision/main.go` via the `matchFunctions` map
- Tickets within a profile are further grouped into `Pool`s for isolated matching

### Redis Data Structure
- **Ticket keys**: `ticket:{ticket_id}` (hash with search fields)
- **Pool keys**: `pool:{profile_name}:{pool_name}` (sorted set of ticket IDs)
- **Assignment channel**: Redis pub/sub used to notify clients of assignments

## Testing

Run with `task test`. Tests use standard Go testing patterns.

## Markdown Lint Compliance Rule

**ABSOLUTE RULE**: When outputting or editing markdown content, always follow markdownlint rules to avoid warnings.

This rule must be followed for all markdown output to ensure consistent formatting and quality.
