// Jest setup provided by Grafana scaffolding
import './.config/jest-setup';

const NativeMessageChannel = global.MessageChannel;
const messageChannels = [];
global.MessageChannel = class extends NativeMessageChannel {
  constructor(...args) {
    super(...args);
    messageChannels.push(this);
  }
};

afterAll(() => {
  for (const channel of messageChannels) {
    channel.port1.close();
    channel.port2.close();
  }
});
