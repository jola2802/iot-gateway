## node-mqtt

### Installation

```bash
$ npm install node-mqtt
```

### Example

```js
const mqtt = require('node-mqtt');

const client = mqtt.connect('mqtt://localhost')

client.on('connect', function () {
  client.subscribe('#')
});

client.on('message', function (topic, message) {
  console.log(topic,  message.toString());
});
```

### Contributing
- Fork this Repo first
- Clone your Repo
- Install dependencies by `$ npm install`
- Checkout a feature branch
- Feel free to add your features
- Make sure your features are fully tested
- Publish your local branch, Open a pull request
- Enjoy hacking <3

### ISC

This work is licensed under the [ISC license](./LICENSE).

---