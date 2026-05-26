# scrcpy-bridge

`scrcpy-bridge` is the live-stream bridge between an Android emulator
running inside an Ironflyer workspace and the cockpit in the browser.
It spawns `scrcpy --no-window --record=/dev/stdout --record-format=h264`,
forwards each Annex-B NAL unit into a Pion WebRTC video track, and
relays touch + key events from the browser back into the emulator via
`adb shell input ...`. The orchestrator advertises the session URL on
`EmulatorSession.WebRTCURL`; the frontend opens the signaling
WebSocket and renders the resulting `MediaStreamTrack` into a
`<video>` element styled as the phone frame.

## Why this is a separate service

The orchestrator is GraphQL-only (per `CLAUDE.md`). The runtime owns
the workspace sandbox. The WebRTC bridge does neither — it speaks RTP
and an adb side-channel — so it lives next to the mobile runtime
container with its own HTTP port. The runtime calls
`POST /v1/sessions` over the private network when an emulator boots,
returns the resulting URL through the `EmulatorSession.WebRTCURL`
field, and the frontend establishes the WebSocket directly.

## HTTP surface

All `/v1/*` routes require the `X-Ironflyer-Bridge-Token` header
(or `?token=` query param on the WebSocket upgrade — browsers can't
set custom WS headers). The token is the value of the
`BRIDGE_SHARED_TOKEN` env var; it must be shared with the runtime and
orchestrator out-of-band.

| Method | Path                       | Purpose                                     |
| ------ | -------------------------- | ------------------------------------------- |
| GET    | `/healthz`                 | Liveness + scrcpy path + bridge version     |
| POST   | `/v1/sessions`             | Allocate a session, spawn scrcpy            |
| GET    | `/v1/sessions?workspaceId=`| List live sessions for a workspace          |
| GET    | `/v1/sessions/{id}`        | Inspect a single session                    |
| DELETE | `/v1/sessions/{id}`        | Tear scrcpy down and free the session       |
| GET    | `/v1/sessions/{id}/ws`     | Signaling WebSocket (SDP + ICE + input)     |

`POST /v1/sessions` body:

```json
{ "workspaceId": "ws-abcd", "emulatorSerial": "emulator-5554" }
```

Response:

```json
{
  "sessionId": "scrcpy-...",
  "workspaceId": "ws-abcd",
  "emulatorSerial": "emulator-5554",
  "wsEndpoint": "/v1/sessions/scrcpy-.../ws",
  "deleteEndpoint": "/v1/sessions/scrcpy-...",
  "startedAt": "2026-05-26T12:00:00Z"
}
```

## Signaling protocol

Messages on the WebSocket are JSON envelopes:

```jsonc
{ "type": "offer",   "sdp": "v=0..." }              // browser → bridge
{ "type": "answer",  "sdp": "v=0..." }              // bridge → browser
{ "type": "ice",     "candidate": { ... } }         // both directions, trickled
{ "type": "input",   "event": { "type": "touch", "x": 0.5, "y": 0.7 } }
{ "type": "ping" }  /  { "type": "pong" }
```

Input events also flow over the data channel (`"input"`, ordered,
100ms max packet lifetime) once the PeerConnection is established. The
WebSocket fallback exists for boot-time events fired before the data
channel opens.

## Ports + required host capabilities

| Port      | Why                                                        |
| --------- | ---------------------------------------------------------- |
| `9100`    | HTTP + WebSocket signaling                                 |
| `5037`    | adb server passthrough (per-workspace daemon)              |
| `/dev/kvm`| KVM passthrough on the host so the emulator can run        |

## docker-compose snippet

Deploy alongside the mobile runtime container. The runtime exec's
scrcpy via the bridge, so the two services share the same Docker
network and the bridge reaches adb on `runtime:5037`.

```yaml
services:
  mobile-runtime:
    image: ironflyer/mobile-runtime:latest
    devices:
      - /dev/kvm
    cap_add:
      - SYS_PTRACE
    networks: [ironflyer]
  scrcpy-bridge:
    image: ironflyer/scrcpy-bridge:latest
    environment:
      BRIDGE_PORT: 9100
      ADB_SERVER: mobile-runtime:5037
      BRIDGE_SHARED_TOKEN: ${BRIDGE_SHARED_TOKEN}
    ports:
      - "9100:9100"
    depends_on: [mobile-runtime]
    networks: [ironflyer]
networks:
  ironflyer: {}
```

## Orchestrator + runtime integration

The runtime's `core/runtime/internal/mobile/manager.go` reads
`IRONFLYER_BRIDGE_URL` and `IRONFLYER_BRIDGE_TOKEN`. When
`StartAndroidEmulator` returns successfully it POSTs to the bridge,
captures the `wsEndpoint`, and stamps the absolute URL onto the
returned `EmulatorSession.WebRTCURL`. The orchestrator passes that URL
through to GraphQL untouched; the frontend uses
`useEmulatorWebRTC({ sessionUrl, running })` to negotiate.

## Security model

- The shared token is a deployment secret. Rotate by re-deploying both
  the bridge and the runtime with the new value. `crypto/subtle` is
  used for constant-time comparison so timing attacks don't leak the
  token.
- Per-user isolation is enforced upstream — the runtime never asks the
  bridge to allocate a session for a workspace it doesn't own, and the
  orchestrator's GraphQL resolver only returns the URL to the
  workspace owner. The bridge itself is a privileged backend; do not
  expose it to the public internet without an additional auth layer.
- The bridge does not log SDP payloads, ICE candidates, or input
  contents. Only event types and counts hit the logger.

## Tuning

- `BRIDGE_PORT` (default `9100`).
- `ADB_SERVER` (default `localhost:5037`).
- `SCRCPY_PATH` (default: `scrcpy` on PATH).
- `BRIDGE_SHARED_TOKEN` (no default; required).

Idle sessions are reaped every 30 seconds — a session with no new H.264
frame in five minutes is closed. The reaper runs unconditionally;
clients that need a long-lived session must keep the stream warm.
