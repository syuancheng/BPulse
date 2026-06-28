const environments = Object.freeze({
  local: Object.freeze({cloudbaseEnvId: "", serviceName: "", mediaParserFunctionName: "media-parser"}),
  test: Object.freeze({
    cloudbaseEnvId: "CONFIGURE_IN_PRIVATE_BUILD",
    serviceName: "bp-api-test",
    mediaParserFunctionName: "media-parser",
  }),
  production: Object.freeze({
    cloudbaseEnvId: "CONFIGURE_IN_PRIVATE_BUILD",
    serviceName: "bp-api-prod",
    mediaParserFunctionName: "media-parser",
  }),
});

function getEnvironment(name) {
  const environment = environments[name];
  if (!environment) {
    throw new Error("Unknown application environment");
  }
  return environment;
}

function getActiveEnvironment() {
  const envVersion =
    typeof wx !== "undefined" && wx.getAccountInfoSync
      ? wx.getAccountInfoSync().miniProgram.envVersion
      : "develop";
  return getEnvironment(envVersion === "release" ? "production" : "test");
}

module.exports = {getEnvironment, getActiveEnvironment};
