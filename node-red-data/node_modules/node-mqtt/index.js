const util         = require('util');
const EventEmitter = require('events');

function MQTT(options){
  if(!(this instanceof MQTT))
    return new MQTT(options);
  EventEmitter.call(this);
  return this;
}

util.inherits(MQTT, EventEmitter)

MQTT.prototype.subscribe = function(topics){
  return this;
};

MQTT.connect = function(options){
  return new MQTT(options);
};
MQTT.Server = require('./server');
MQTT.createServer = function(options){
  return new MQTT.Server(options);
};

module.exports = MQTT;