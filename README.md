# ottam

[Pronounced - Oh-ttum]

Websocket pooler for realtime systems.

## Goals

### Phase - 1

- multiple clients per user can connect to the server through ottam
- ottam to support bi-directional communication
- minimal delivery guarantee (atMostOnce)
- horizontally scllable
- server and ottam is decouopled through a message queue (only valkey pub/sub initially)
