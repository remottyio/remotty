# Remotty 

A web-based terminal management server that allows you to register terminal sessions and connect to them through a browser using WebRTC.

## Features

- **Automatic WebRTC Connection**: No manual SDP copy/paste required
- **Web UI**: Clean Bootstrap-based interface with xterm.js terminal
- **Real-time Terminal**: Full PTY support with bidirectional I/O
- **Registration System**: Hosts register and wait for browser connections
- **Long Polling**: Server holds requests for up to 10 seconds for efficient connection handling
- **Automatic Cleanup**: Inactive agents are automatically removed after 15 seconds
- **RESTful API**: JSON API for programmatic access
- **Structured Logging**: JSON logging with logrus

## Installation

### Prerequisites

- Go 1.20 or later

### Build from Source

```bash
git clone <repository-url>
cd remotty
go build
```

## Usage

### Starting the Server

```bash
# Default port (8080)
./remotty server start

# Custom port
./remotty server start --port 3000
```

The server will start on `http://localhost:8080` (or your specified port).

### Registering a Terminal

From the remotty directory:

```bash
remotty agent register --id my-laptop --host http://localhost:8080
```

The terminal will:
1. Create a WebRTC offer
2. Register with the server
3. Wait for a browser connection (60 seconds)
4. Start a bash shell when connected

### Connecting via Browser

1. Open `http://localhost:8080` in your browser
2. Click **Connect** on any registered host
3. A fullscreen terminal opens with an interactive bash session

## API Endpoints

## API Version 1 (`/api/1/`)

### POST /api/1/register

Register a new terminal session.

**Request:**
```json
{
  "id": "my-laptop",
  "sdp": "v=0\r\no=- 123456789 2 IN IP4 127.0.0.1\r\n..."
}
```

**Response:** `201 Created`
```json
{
  "status": "registered",
  "id": "my-laptop"
}
```

### GET /api/1/list

List all registered hosts in JSON format.

**Response:**
```json
[
  {
    "id": "my-laptop",
    "registered_at": "2024-01-01T12:00:00Z",
    "remote_addr": "192.168.1.100"
  }
]
```

### GET /api/1/connect/:id

Get SDP offer for a host.

**Response:**
```json
{
  "id": "my-laptop",
  "sdp": "v=0\r\no=- 123456789 2 IN IP4 127.0.0.1\r\n..."
}
```

### POST /api/1/answer/:id

Submit WebRTC answer from browser.

**Request:**
```json
{
  "answer": "raw-sdp-answer-string"
}
```

### GET /api/1/answer/:id

Long poll for answer (used by terminal client). Server waits up to 10 seconds for an answer before returning `204 No Content` if none is available.

## Architecture

```
┌─────────────┐                  ┌───────────┐                        ┌──────────┐
│  Terminal   │                  │  Remotty  │                        │ Browser  │
│  (remotty)  │                  │   Server  │                        │          │
└─────────────┘                  └───────────┘                        └──────────┘
       │                                 │                                   │
       │  1. POST /api/1/register        │                                   │
       │  {id, sdp-offer}                │                                   │
       ├────────────────────────────────>│                                   │
       │                                 │                                   │
       │  2. Long poll /api/1/answer/:id │                                   │
       │  (waits up to 10s on server)    │                                   │
       ├────────────────────────────────>│                                   │
       │    204 No Content (after 10s)   │                                   │
       │<────────────────────────────────┤                                   │
       │                                 │                                   │
       │                                 │  3. GET /                         │
       │                                 │<──────────────────────────────────┤
       │                                 │  (HTML with host list)            │
       │                                 ├──────────────────────────────────>│
       │                                 │                                   │
       │                                 │ 4. GET /api/1/connect/:id?raw=true│
       │                                 │<──────────────────────────────────┤
       │                                 │  {sdp-offer}                      │
       │                                 ├──────────────────────────────────>│
       │                                 │                                   │
       │                                 │  5. POST /api/1/answer/:id        │
       │                                 │<──────────────────────────────────┤
       │                                 │  {answer}                         │
       │                                 ├──────────────────────────────────>│
       │                                 │                                   │
       │  6. Poll GET /api/1/answer/:id  │                                   │
       ├────────────────────────────────>│                                   │
       │    200 OK {answer}              │                                   │
       │<────────────────────────────────┤                                   │
       │                                 │                                   │
       │                     7. WebRTC Connection Established                │
       │<────────────────────────────────────────────────────────────────────┤
       │                                                                     │
       │             8. Bidirectional Terminal I/O over WebRTC               │
       │<───────────────────────────────────────────────────────────────────>│
```

## Security Considerations

- **No Authentication**: This version has no authentication. Add authentication before exposing to the internet.
- **In-Memory Storage**: Host registrations are not persistent. Registered host are lost on server restart.
- **STUN Server**: Uses public Google STUN server. Consider running your own for production.
- **No Encryption**: SDP is not encrypted during registration. Use HTTPS in production.

## License

Apache License - see LICENSE file for details

## Contributing

Contributions welcome! Please open an issue or pull request.

## Acknowledgments

- Heavily inspired by [webtty](https://github.com/maxmcd/webtty) by Max McDonnell
- Uses [xterm.js](https://xtermjs.org/) for terminal emulation
- Uses [Bootstrap](https://getbootstrap.com/) for UI
