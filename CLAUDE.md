# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

All commands use `task` (Taskfile.yml):

- `task build` - Build collision server and simpleticket client binaries to `./bin/`
- `task run` - Run collision server directly with `go run`
- `task run-client` - Run simpleticket client with `go run`
- `task dev` - Build and run the collision server
- `task proto` - Generate Go files from Protocol Buffer definitions (required after any `.proto` changes)
- `task clean` - Remove build artifacts
- `task test` - Run all tests with `go test -v ./...`
- `task check` - Run formatting, vet, and tests
- `task fmt` - Format code with `go fmt ./...`
- `task vet` - Run `go vet ./...`
- `task mod-tidy` - Tidy dependencies with `go mod tidy`
- `task build-all` - Build binaries for Linux, macOS (amd64/arm64), and Windows

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

**Match Function** (`cmd/collision/main.go:114`) - Implement `MatchFunctionSimple1vs1` or register new functions in the `matchFunctions` map:

- Input: Active tickets grouped by pool
- Output: List of `Match` objects (each contains matched tickets)
- Simple1vs1 example: pairs tickets in groups of 2

**Assigner Function** (`cmd/collision/main.go:138`) - Implement `dummyAssign` or replace in DI:

- Input: List of matches
- Output: `AssignmentGroup` objects with server connection strings
- Currently generates random server names via `hri.Random()`

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

## Development Notes

- **Redis requirement**: Server requires Redis running on `127.0.0.1:6379` (hardcoded in `infrastructure/redis.go:13`)
- **Default port**: gRPC server listens on `127.0.0.1:31080` (see `cmd/collision/main.go:21`)
- **Match interval**: Matching logic executes every 1 second (see `cmd/collision/main.go:96`)
- **Ticket TTL**: Newly created tickets expire in 10 minutes (see `usecase/ticket.go:36`)
- **Lock TTL**: Redis locks have 1-second timeout (see `infrastructure/redis.go:10`)

## Testing

Run with `task test`. Tests use standard Go testing patterns.
