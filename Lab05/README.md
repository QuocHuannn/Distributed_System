# Lab05 - Distributed Key-Value Storage System with Consistent Hashing

A distributed key-value storage system using consistent hashing to distribute data across multiple servers.

## Overview

This lab implements a distributed key-value storage system utilizing consistent hashing to distribute data across multiple server nodes. The system provides fault tolerance, high availability, and automatic data redistribution when servers join or leave the network.

## Features

- **Distributed Storage**: Data is distributed across multiple servers.
- **Consistent Hashing**: Efficient data distribution with minimal redistribution when the system structure changes.
- **Fault Tolerance**: The system remains operational even if some servers fail.
- **Transparent Routing**: Clients can connect to any server in the cluster.
- **Basic Operations**: Supports GET, SET, and DELETE operations.

## Implementation Details

### Consistent Hashing

The system uses consistent hashing to determine which server is responsible for storing each key. This approach offers several advantages:
- Only K/N keys need to be remapped when a server joins or leaves (where K is the number of keys and N is the number of servers).
- The hash space is represented as a circular ring, with servers and keys mapped to positions on the ring.
- Each key is assigned to the first server encountered when moving clockwise from the key's position.

### Server Implementation

Each server:
- Maintains a portion of the key-value data.
- Is aware of all other servers in the system.
- Forwards requests to the appropriate server when it is not responsible for a key.
- Participates in data redistribution when the system structure changes.

### Client Implementation

The client:
- Connects to any server in the system.
- Sends GET, SET, and DELETE requests.
- Receives responses, which may have been forwarded through multiple servers.

## Message Protocol

The system uses a simple JSON-based communication protocol:

```json
{
  "type": "GET|SET|DELETE|RESPONSE",
  "key": "string",
  "value": "string",
  "timestamp": "time",
  "status": "SUCCESS|ERROR",
  "error": "error message"
}
```

## Running the System

### Starting Servers

```bash
# Start the first server
go run server/server.go -id server1 -addr localhost:8081

# Start additional servers
go run server/server.go -id server2 -addr localhost:8082 -join localhost:8081
go run server/server.go -id server3 -addr localhost:8083 -join localhost:8081
```

### Using the Client

```bash
# Connect to any server in the cluster
go run client/client.go -server localhost:8081

# In the client's command line interface:
# Set a key-value pair
> SET mykey myvalue

# Retrieve a value
> GET mykey

# Delete a key
> DELETE mykey

# Exit the client
> EXIT
```

## Running Tests

Integration tests verify the system's functionality, including:
- Basic SET and GET operations
- Data distribution across multiple servers
- System behavior when a server fails

```bash
# Run all tests
go test -v

# Run a specific test
go test -v -run TestSetAndGet
```

## Design Considerations

- **Scalability**: The system can scale horizontally by adding more servers.
- **Consistency**: The system provides eventual consistency.
- **Availability**: The system remains available even if some servers fail.
- **Partition Tolerance**: The system can handle network partitions.

