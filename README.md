# PrimaryBackup-KeyValue Implementation

## Project Overview

This project implements a fault-tolerant key/value storage service using a primary/backup replication model. The system provides consistent key/value operations even in the face of server failures, while ensuring at-most-once semantics for all operations.

**Key features:**
- Replicated key/value store with automatic failover
- Primary/backup server architecture
- Monitoring service integration for failure detection
- At-most-once operation semantics
- Client-side retry logic for fault tolerance

## System Components

### 1. Key/Value Service
- Maintains a map of string key/value pairs
- Supports three operations:
  - Get(key): Retrieves the current value for a key
  - Put(key, value): Sets the value for a key
  - PutHash(key, value): Updates value using hash function and returns previous value

### 2. Monitoring Service
- Tracks server availability
- Determines primary and backup servers
- Provides view updates to clients and servers
- Prevents split-brain scenarios

### 3. Primary/Backup Replication
- Primary handles all client requests
- Backup receives state updates from primary
- Automatic failover when primary fails
- State synchronization for new backups

## Implementation Details

### Server Implementation (server.go)

#### Role Management
- Servers ping monitoring service regularly (tick() function)
- Update their role (primary/backup/none) based on view
- Primary synchronizes state with new backups
- Servers step down when they lose primary status

#### Data Replication
- BackupSync RPC: Full state synchronization to new backups
- PrimaryToBackupPut RPC: Forwarded Put operations
- PrimaryToBackupGet RPC: Forwarded Get operations
- Deep copying of maps to prevent reference sharing

#### Operation Handling
- Duplicate detection using client IDs and sequence numbers
- Proper error handling for wrong server scenarios
- Hash computation for PutHash operations
- State maintenance in memory (no disk persistence)

### Client Implementation (client.go)

#### Operation Execution
- Retry logic with 30-second timeout for all operations
- Periodic view updates when needed
- Only sends requests to current primary
- Includes client ID and sequence numbers in requests

#### Fault Tolerance
- Automatic view updates when primary fails
- Sleep between retries to prevent CPU overload
- Proper handling of temporary failures

### Common Components (common.go)

#### Data Structures
- RPC argument and reply structures
- Error constants
- Operation types (PutArgs, GetArgs, etc.)

#### Utilities
- Hash function for PutHash operations
- Random number generator for client IDs

## Development Flow

1. **Basic Operation Handling**:
   - Implemented Put, Get, and PutHash operations
   - Basic key/value storage in memory

2. **Primary/Backup Management**:
   - Added role detection and state synchronization
   - Implemented view processing in tick() function

3. **Replication**:
   - Added operation forwarding to backup
   - Implemented full state synchronization

4. **Fault Tolerance**:
   - Added client retry logic
   - Implemented duplicate detection
   - Added proper error handling

5. **Optimization**:
   - Reduced monitoring service overhead
   - Efficient state copying
   - Proper resource cleanup


## Requirements
- Linux-based environment (does not work on Windows)
- Go programming language
