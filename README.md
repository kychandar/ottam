# Ottam

> *Pronounced: Oh-ttum*

A high-performance, horizontally scalable WebSocket pooler and real-time message fan-out service built with Go.

## Overview

Ottam is a production-ready WebSocket infrastructure layer designed to handle massive concurrent connections while maintaining low-latency message delivery. It acts as a bridge between backend services and thousands of real-time clients, efficiently distributing messages through a pub/sub architecture.

**🎥 Demo Video:** [Ottam Stress Test Demo - 10 Nov 2025](https://youtu.be/VM9XRmJ1mLg)

## Key Features

### ✅ Available Now

- **Horizontally Scalable** - Seamlessly scale across multiple pods/nodes
- **High Concurrency** - Handle 20,000+ concurrent WebSocket connections per cluster
- **Low Latency** - Sub-30ms average message delivery, P99 < 63ms
- **Bi-directional Communication** - Full duplex WebSocket support
- **Channel-based Subscriptions** - Clients can subscribe to specific message channels
- **Decoupled Architecture** - Backend services publish via message queue (NATS JetStream)

### 🚧 Planned Features

- **Message Replay** - 2-minute replay buffer for late joiners
- **Authentication & Authorization** - Secure connection management
- **Rate Limiting** - Per-client and per-channel rate controls
- **Dedicated Go Client Library** - First-class Go client SDK

## Architecture

![Architecture Diagram](docs/arch.svg)

## Performance Benchmarks

Ottam has been rigorously stress-tested in a Kubernetes environment to validate limits.

### Stress Test Results

#### Test Configuration

| Parameter | Value |
|-----------|-------|
| **Infrastructure** | 4 Kubernetes pods |
| **Resources per Pod** | 4 CPU cores, 8 GiB RAM |
| **Connections per Pod** | ~5,000 WebSocket connections |
| **Total Concurrent Connections** | **20,000 clients** |
| **Test Duration** | 30 seconds |
| **Message Publish Rate** | Every 200ms |
| **Total Messages Delivered** | **2,977,555** (~3M messages) |

#### Latency Distribution

| Percentile | Latency (ms) | Status |
|------------|--------------|--------|
| Average | 29.674 | ✅ |
| P70 | 37.547 | ✅ |
| P75 | 38.981 | ✅ |
| P80 | 40.428 | ✅ |
| P85 | 41.807 | ✅ |
| P90 | 43.131 | ✅ |
| **P95** | **45.162** | **✅** |
| **P99** | **62.474** | **✅** |
| P99.9 | 187.021 | ⚠️ |
| P99.99 | 242.128 | ⚠️ |
| Maximum | 244.137 | ⚠️ |

#### Performance Highlights

- ✅ **20,000 concurrent WebSocket connections** handled successfully across 4 pods
- ✅ **Average latency of 29.67ms** for end-to-end message delivery
- ✅ **P99 latency under 63ms** - 99% of messages delivered in under 63ms
- ✅ **~100K messages/second throughput** (99,252 msg/s sustained)
- ✅ **Near 3 million messages** processed without failures in 30 seconds
- ✅ **Linear scalability** - ~5K connections per pod with consistent performance

## Use Cases

Ottam is ideal for applications requiring real-time data distribution:

- 📊 **Live Dashboards** - Real-time analytics and monitoring
- 💬 **Chat Applications** - Instant messaging and notifications
- 🎮 **Gaming** - Multiplayer game state synchronization
- 📈 **Financial Data** - Stock tickers, trading updates
- 🚨 **Alert Systems** - Real-time notifications and alerts
- 📡 **IoT** - Device telemetry and command distribution

## Getting Started

```bash
# Clone the repository
git clone https://github.com/kychandar/ottam.git

# Build the application
cd ottam
go build -o ottam ./

# Run the server
./ottam server
```

## Configuration

Environment variables and configuration details coming soon.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

**Built with ❤️ for high-performance real-time systems**
