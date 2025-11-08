import ws from 'k6/ws';
import { fail } from 'k6';
import { Trend } from 'k6/metrics';
import { check, sleep } from 'k6';

const wsLatency = new Trend('ws_message_latency', true);

export const options = {
  summaryTrendStats: ['min', 'avg', 'med', 'max', 'p(75)', 'p(90)', 'p(95)', 'p(99)'],
  vus: 1,       // number of virtual users
  duration: '30s' // test duration
};


const targetAddr = __ENV.TARGET_ADDR;
if (!targetAddr) {
  fail('Environment variable TARGET_ADDR is not set!');
}


export default function () {
  // const url = 'ws://localhost:8080/ws';
  const url = `${targetAddr}/ws`;
  // const url = 'ws://100.130.101.125:30080/ws';

  const res = ws.connect(url, null, function (socket) {
    socket.on('open', function open() {
      // console.log('connected');
      socket.send('{"Version":1,"Id":"abc123","IsControlPlaneMessage":true,"Payload":{"Version":1,"ControlPlaneOp":0,"Payload":{"ChannelName":"t1"}}}');
    });



    // socket.on('binaryMessage', function (data) {
    //   // Convert binary -> string
    //   const msg = decodeUtf8(data); // e.g., "t1:1698412345678901234"
    //   console.log("msg", msg);
    //   if (msg.startsWith("t1:")) {
    //     // Extract timestamp in nanoseconds
    //     const timestampNs = BigInt(msg.substring(3).trim());

    //     // Convert to milliseconds
    //     const sentTimeMs = Number(timestampNs / 1000000n);

    //     // Compute latency
    //     const latencyMs = Date.now() - sentTimeMs;

    //     wsLatency.add(latencyMs);
    //     // console.log(`latency: ${latencyMs} ms`);
    //   } else {
    //     console.error("⚠️ Unrecognized format:", msg);
    //   }
    // });

    socket.on('binaryMessage', function (data) {
      try {
        // Convert binary to string
        const msgStr = String.fromCharCode.apply(null, new Uint8Array(data));
        const msg = JSON.parse(msgStr);

        // Extract published_time (nanoseconds)
        if (msg.published_time) {
          // Convert nanoseconds to milliseconds
          const sentTimeMs = Number(BigInt(msg.published_time) / 1000000n);

          // Calculate latency
          const latencyMs = Date.now() - sentTimeMs;

          wsLatency.add(latencyMs);

          // Optional: log sample
          if (Math.random() < 0.01) {
            console.log(`Channel: ${msg.channel_name}, Latency: ${latencyMs}ms, MsgID: ${msg.msg_id}`);
          }
        } else {
          console.error('Missing published_time in message');
        }
      } catch (e) {
        console.error('Binary parse error:', e.message, 'Data:', data);
      }
    });



    socket.on('error', (e) => console.error('WebSocket error:', e));
    // socket.on('close', () => console.log('disconnected'));

    socket.setTimeout(function () {
      socket.close();
    }, 30000);

  });



  check(res, { 'status is 101': (r) => r && r.status === 101 });
}

