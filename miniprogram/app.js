const config = require("./config/index");

App({
  onLaunch() {
    const environment = config.getActiveEnvironment();
    if (wx.cloud && environment.cloudbaseEnvId) {
      wx.cloud.init({env: environment.cloudbaseEnvId});
    }
  },
});
