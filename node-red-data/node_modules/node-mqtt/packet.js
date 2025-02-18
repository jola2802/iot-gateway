
function MQTTPacket(){

}

MQTTPacket.parse = function(stream){
  const packet = new MQTTPacket();
  // parse
  return packet;
};

MQTTPacket.prototype.toBuffer = function(){
  const buffer = [];
  return new Buffer(buffer);
};

module.exports = MQTTPacket;