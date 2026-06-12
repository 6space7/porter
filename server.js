const http = require('http');
const net = require('net');

function checkDatabase(callback) {
  const raw = process.env.DATABASE_URL || '';
  if (!raw) return callback('missing database url');
  let parsed;
  try {
    parsed = new URL(raw);
  } catch (err) {
    return callback('invalid database url');
  }
  const port = Number(parsed.port || 5432);
  const socket = net.createConnection({ host: parsed.hostname, port, timeout: 2000 }, () => {
    socket.destroy();
    callback('db reachable');
  });
  socket.on('error', () => callback('db error'));
  socket.on('timeout', () => {
    socket.destroy();
    callback('db timeout');
  });
}

http.createServer((req, res) => {
  checkDatabase((status) => {
    res.writeHead(200, { 'content-type': 'text/plain' });
    res.end(`porter phase4 nixpacks ${status}`);
  });
}).listen(process.env.PORT || 3000, '0.0.0.0');