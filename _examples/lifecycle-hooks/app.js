const http = require('http');

const handler = (req, res) => {
  console.log(`${req.method} ${req.url}`);

  switch (req.url) {
    case '/aws/lambda-microvms/runtime/v1/ready':
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'ready' }));
      return;
    case '/aws/lambda-microvms/runtime/v1/validate':
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'validated' }));
      return;
    default:
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'ok', path: req.url }));
  }
};

http.createServer(handler).listen(8080, '0.0.0.0', () => {
  console.log('Listening on 0.0.0.0:8080');
});

http.createServer(handler).listen(9000, '0.0.0.0', () => {
  console.log('Listening on 0.0.0.0:9000 (hooks)');
});
