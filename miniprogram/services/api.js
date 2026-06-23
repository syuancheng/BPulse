function createApiClient({cloudbaseEnvId, serviceName}) {
  if (!cloudbaseEnvId || !serviceName) {
    throw new Error("CloudBase API configuration is required");
  }

  return {
    async call({path, method = "GET", data}) {
      const response = await wx.cloud.callContainer({
        config: {env: cloudbaseEnvId},
        path,
        method,
        data,
        header: {
          "X-WX-SERVICE": serviceName,
          "content-type": "application/json",
        },
      });
      if (response.statusCode < 200 || response.statusCode >= 300) {
        throw new Error(`API request failed with status ${response.statusCode}`);
      }
      return response.data;
    },
    createBPRecord(data) {
      return this.call({path: "/api/v1/bp-records", method: "POST", data});
    },
  };
}

module.exports = {createApiClient};
