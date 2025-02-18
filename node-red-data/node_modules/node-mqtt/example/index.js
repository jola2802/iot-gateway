const mqtt = require('..');

const client = mqtt.connect('mqtt://localhost')

client.on('connect', function () {
  client.subscribe('#')
});

client.on('message', function (topic, message) {
  console.log(topic,  message.toString());
});