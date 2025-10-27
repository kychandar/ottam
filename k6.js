import ws from 'k6/ws';
import { Trend } from 'k6/metrics';
import { check, sleep } from 'k6';

const wsLatency = new Trend('ws_message_latency', true);

export const options = {
  vus: 100,       // number of virtual users
  duration: '10s' // test duration
};

export default function () {
  const url = 'ws://localhost:8080/ws';

  const res = ws.connect(url, null, function (socket) {
    socket.on('open', function open() {
      // console.log('connected');
      socket.send('{"Version":1,"Id":"abc123","IsControlPlaneMessage":true,"Payload":{"Version":1,"ControlPlaneOp":0,"Payload":{"ChannelName":"t1"}}}');

    });



    socket.on('binaryMessage', function (data) {
      // Convert binary -> string
      const msg = decodeUtf8(data); // e.g., "t1:1698412345678901234"

      if (msg.startsWith("t1:")) {
        // Extract timestamp in nanoseconds
        const timestampNs = BigInt(msg.substring(3).trim());

        // Convert to milliseconds
        const sentTimeMs = Number(timestampNs / 1000000n);

        // Compute latency
        const latencyMs = Date.now() - sentTimeMs;

        wsLatency.add(latencyMs);
        // console.log(`latency: ${latencyMs} ms`);
      } else {
        console.error("⚠️ Unrecognized format:", msg);
      }
    });


    socket.on('error', (e) => console.error('WebSocket error:', e));
    // socket.on('close', () => console.log('disconnected'));

    socket.setTimeout(function () {
      socket.close();
    }, 10000);

  });



  check(res, { 'status is 101': (r) => r && r.status === 101 });
}


/*
export default function () {
  const url = 'wss://echo.websocket.org';
  const params = { tags: { my_tag: 'hello' } };

  const res = ws.connect(url, params, function (socket) {
    socket.on('open', () => console.log('connected'));
    socket.on('message', (data) => console.log('Message received: ', data));
    socket.on('close', () => console.log('disconnected'));
  });

  check(res, { 'status is 101': (r) => r && r.status === 101 });
}
  */

function decodeUtf8(arrayBuffer) {
  const bytes = new Uint8Array(arrayBuffer);
  let str = '';
  for (let i = 0; i < bytes.length; i++) {
    str += String.fromCharCode(bytes[i]);
  }
  try {
    // Convert possible UTF-8 multi-byte chars
    return decodeURIComponent(escape(str));
  } catch (e) {
    return str; // fallback for pure ASCII
  }
}