// useEmulatorWebRTC — opens the scrcpy-bridge signaling WebSocket,
// negotiates an SDP offer/answer pair with the bridge's Pion peer
// connection, and exposes the incoming H.264 video track + a
// sendInput helper to the EmulatorStreamView component.
//
// The bridge service speaks JSON envelopes over the WS:
//   { type: "offer",  sdp }      browser → bridge
//   { type: "answer", sdp }      bridge  → browser
//   { type: "ice",    candidate }  both directions, trickled
//   { type: "input",  event }    browser → bridge (input fallback)
//
// Once the data channel ("input") opens, sendInput prefers it over
// the WebSocket so latency stays minimal.

"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export type EmulatorInputEvent =
  | {
      type: "touch";
      x: number; // normalised [0, 1]
      y: number;
    }
  | {
      type: "swipe";
      x: number;
      y: number;
      x2: number;
      y2: number;
      duration?: number;
    }
  | {
      type: "key";
      keycode: number;
    }
  | {
      type: "text";
      text: string;
    };

export interface UseEmulatorWebRTCOptions {
  sessionUrl?: string;
  running: boolean;
}

export interface UseEmulatorWebRTCResult {
  videoRef: React.RefObject<HTMLVideoElement | null>;
  sendInput: (ev: EmulatorInputEvent) => void;
  connected: boolean;
  error?: string;
}

const STUN_URL = "stun:stun.l.google.com:19302";

export function useEmulatorWebRTC({
  sessionUrl,
  running,
}: UseEmulatorWebRTCOptions): UseEmulatorWebRTCResult {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const dataChannelRef = useRef<RTCDataChannel | null>(null);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const sendInput = useCallback((ev: EmulatorInputEvent) => {
    const payload = JSON.stringify({ type: "input", event: ev });
    const dc = dataChannelRef.current;
    if (dc && dc.readyState === "open") {
      try {
        dc.send(JSON.stringify(ev));
        return;
      } catch {
        /* fall through to WS */
      }
    }
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      try {
        ws.send(payload);
      } catch {
        /* swallow — the next frame will retry */
      }
    }
  }, []);

  useEffect(() => {
    if (!running || !sessionUrl) {
      return;
    }

    let cancelled = false;
    setError(undefined);
    setConnected(false);

    const pc = new RTCPeerConnection({
      iceServers: [{ urls: STUN_URL }],
    });
    pcRef.current = pc;

    pc.addTransceiver("video", { direction: "recvonly" });

    pc.ontrack = (event) => {
      const stream = event.streams[0] ?? new MediaStream([event.track]);
      const el = videoRef.current;
      if (el && el.srcObject !== stream) {
        el.srcObject = stream;
        el.play().catch(() => {
          /* autoplay restrictions; the muted attribute should cover it */
        });
      }
    };

    pc.onconnectionstatechange = () => {
      if (cancelled) return;
      if (pc.connectionState === "connected") {
        setConnected(true);
      } else if (
        pc.connectionState === "failed" ||
        pc.connectionState === "disconnected" ||
        pc.connectionState === "closed"
      ) {
        setConnected(false);
      }
    };

    const ws = new WebSocket(sessionUrl);
    wsRef.current = ws;

    pc.onicecandidate = (ev) => {
      if (!ev.candidate) {
        return; // end-of-candidates marker
      }
      if (ws.readyState !== WebSocket.OPEN) {
        return;
      }
      try {
        ws.send(
          JSON.stringify({
            type: "ice",
            candidate: ev.candidate.toJSON(),
          }),
        );
      } catch {
        /* socket closed mid-flight */
      }
    };

    ws.onopen = async () => {
      if (cancelled) return;
      try {
        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);
        ws.send(JSON.stringify({ type: "offer", sdp: offer.sdp }));
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      }
    };

    ws.onmessage = async (event) => {
      if (cancelled) return;
      let msg: { type?: string; sdp?: string; candidate?: RTCIceCandidateInit; error?: string } = {};
      try {
        msg = JSON.parse(typeof event.data === "string" ? event.data : "");
      } catch {
        return;
      }
      switch (msg.type) {
        case "answer":
          if (msg.sdp) {
            try {
              await pc.setRemoteDescription({ type: "answer", sdp: msg.sdp });
            } catch (err) {
              setError(err instanceof Error ? err.message : String(err));
            }
          }
          break;
        case "ice":
          if (msg.candidate) {
            try {
              await pc.addIceCandidate(msg.candidate);
            } catch {
              /* candidate races are expected — ignore */
            }
          }
          break;
        case "error":
          if (msg.error) {
            setError(msg.error);
          }
          break;
        default:
          break;
      }
    };

    ws.onerror = () => {
      if (cancelled) return;
      setError("Signaling WebSocket error.");
    };

    ws.onclose = () => {
      if (cancelled) return;
      setConnected(false);
    };

    // The bridge advertises an "input" data channel from its side, so
    // we listen for it rather than creating one locally. The browser
    // pc.ondatachannel fires once the offer/answer + ICE complete.
    pc.ondatachannel = (event) => {
      const dc = event.channel;
      if (dc.label !== "input") return;
      dataChannelRef.current = dc;
      dc.onclose = () => {
        if (dataChannelRef.current === dc) {
          dataChannelRef.current = null;
        }
      };
    };

    return () => {
      cancelled = true;
      try {
        ws.close();
      } catch {
        /* ignore */
      }
      try {
        pc.close();
      } catch {
        /* ignore */
      }
      wsRef.current = null;
      pcRef.current = null;
      dataChannelRef.current = null;
      const el = videoRef.current;
      if (el) {
        el.srcObject = null;
      }
    };
  }, [running, sessionUrl]);

  return { videoRef, sendInput, connected, error };
}
