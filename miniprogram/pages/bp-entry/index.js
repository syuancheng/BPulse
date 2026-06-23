const {createApiClient} = require("../../services/api");
const config = require("../../config/index");

function pad(value) {
  return String(value).padStart(2, "0");
}

function newClientRequestId() {
  return `bp-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function localParts(date) {
  return {
    date: `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`,
    time: `${pad(date.getHours())}:${pad(date.getMinutes())}`,
  };
}

function timezoneOffset(date) {
  const offsetMinutes = -date.getTimezoneOffset();
  const sign = offsetMinutes >= 0 ? "+" : "-";
  const absolute = Math.abs(offsetMinutes);
  return `${sign}${pad(Math.floor(absolute / 60))}:${pad(absolute % 60)}`;
}

function rfc3339FromLocal(datePart, timePart) {
  return `${datePart}T${timePart}:00${timezoneOffset(new Date(`${datePart}T${timePart}:00`))}`;
}

Page({
  data: {
    date: "",
    time: "",
    systolic1: "",
    diastolic1: "",
    pulse1: "",
    showSecond: false,
    systolic2: "",
    diastolic2: "",
    pulse2: "",
    note: "",
    status: "idle",
    error: "",
    clientRequestId: "",
    timezone: "Asia/Singapore",
    timeCaptured: false,
  },

  onLoad() {
    this.api = createApiClient(config.getActiveEnvironment());
    this.startDraft();
  },

  startDraft() {
    const now = new Date();
    const parts = localParts(now);
    this.setData({
      date: parts.date,
      time: parts.time,
      clientRequestId: newClientRequestId(),
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
      timeCaptured: false,
      error: "",
    });
  },

  captureFreshTimeIfNeeded() {
    if (this.data.timeCaptured) return;
    const parts = localParts(new Date());
    this.setData({date: parts.date, time: parts.time, timeCaptured: true});
  },

  onFieldInput(event) {
    this.captureFreshTimeIfNeeded();
    this.setData({[event.currentTarget.dataset.field]: event.detail.value, error: ""});
  },

  onDateChange(event) {
    this.setData({date: event.detail.value, timeCaptured: true, error: ""});
  },

  onTimeChange(event) {
    this.setData({time: event.detail.value, timeCaptured: true, error: ""});
  },

  addSecondReading() {
    this.captureFreshTimeIfNeeded();
    this.setData({showSecond: true});
  },

  buildReadings() {
    const first = this.buildReading("1");
    if (!first) return null;
    const readings = [first];
    if (this.data.showSecond) {
      const second = this.buildReading("2");
      if (!second) return null;
      readings.push(second);
    }
    return readings;
  },

  buildReading(suffix) {
    const systolic = Number(this.data[`systolic${suffix}`]);
    const diastolic = Number(this.data[`diastolic${suffix}`]);
    const pulseRaw = this.data[`pulse${suffix}`];
    if (!Number.isInteger(systolic) || !Number.isInteger(diastolic)) {
      this.setData({error: "请填写整数形式的高压和低压。"});
      return null;
    }
    const reading = {systolic, diastolic};
    if (pulseRaw !== "") {
      const pulse = Number(pulseRaw);
      if (!Number.isInteger(pulse)) {
        this.setData({error: "心率请填写整数，或留空。"});
        return null;
      }
      reading.pulse = pulse;
    }
    return reading;
  },

  async save() {
    if (this.data.status === "saving") return;
    this.captureFreshTimeIfNeeded();
    const readings = this.buildReadings();
    if (!readings) return;
    this.setData({status: "saving", error: ""});
    try {
      await this.api.createBPRecord({
        clientRequestId: this.data.clientRequestId,
        measuredAt: rfc3339FromLocal(this.data.date, this.data.time),
        timezone: this.data.timezone,
        entryMethod: "manual",
        readings,
        note: this.data.note || undefined,
      });
      this.setData({status: "saved"});
      wx.showToast({title: "已保存", icon: "success"});
      wx.navigateBack();
    } catch (error) {
      this.setData({status: "error", error: "保存失败，请检查网络后重试。"});
    }
  },
});
