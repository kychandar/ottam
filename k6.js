import ws from 'k6/ws';
import { check, sleep } from 'k6';
import http from 'k6/http';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

const MESSAGE_API = 'https://zenquotes.io/api/random';


export const options = {
  vus: 20000,
  duration: '30s',
};

export default function () {
  const url = 'ws://localhost:8080/ws'; // Adjust to your endpoint

  const res = ws.connect(url, null, function (socket) {
    socket.on('open', function () {
      console.log('Connected to WebSocket server');

    for (let i = 0; i < 5; i++) {
            // let msgRes = http.get(MESSAGE_API);
            let messageText = 'default message';
            // if (msgRes.status === 200) {
            //     try {
            //         let json = msgRes.json();
            //         messageText = json[0].q || messageText;
            //            const payload = JSON.stringify({
            //               vu: __VU,
            //               message: messageText,
            //           });
            //         socket.send(payload);
            //     } catch (e) {
            //         console.error(`VU ${__VU} failed to parse JSON: ${e} ${msgRes.status}`);
            //     }
            // }

            const payload = JSON.stringify({
                          vu: __VU,
                          message: messageText,
                      });
                    socket.send(payload);
            // sleep(randomIntBetween(1, 3));
          }

      // Wait a few seconds before closing
      socket.setTimeout(() => {
        console.log('Closing connection');
        socket.close();
      }, 5);
    });

    socket.on('message', function (message) {
      console.log(`Received: ${message}`);
    });

    socket.on('close', function () {
      console.log('Connection closed');
    });

    socket.on('error', function (e) {
      console.error('Socket error:', e);
    });
  });

  check(res, { 'status is 101 (switching protocols)': (r) => r && r.status === 101 });

  // Give time for WebSocket activity
  // sleep(30);
}
