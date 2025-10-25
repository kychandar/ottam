# ottam

[Pronounced - Oh-ttum]

Websocket pooler for realtime systems.

## Architecture for downstream messages

![Architecture Diagram](docs/arch.svg)

## Goals

### Phase - 1

- multiple clients can connect to the server through ottam
- ottam to support bi-directional communication
- minimal delivery guarantee (atMostOnce)
- horizontally scllable
- server and ottam is decouopled through a message queue (only valkey pub/sub initially)
- client should be able to subscribe to a particular list of channels
