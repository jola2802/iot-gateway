const tcp        = require('net');
const Connection = require('./lib/connection');

function Server(){
  this.clients = [];
  this.server = tcp.createServer();
  this.server.on('connection', socket => {
    const client = new Connection(socket);
    this.clients.push(client);
  });
}

module.exports = Server;