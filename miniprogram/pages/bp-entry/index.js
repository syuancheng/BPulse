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

function randomCloudPath(prefix, extension) {
  return `bp-entry/${prefix}/${Date.now()}-${Math.random().toString(16).slice(2)}.${extension}`;
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
    assistMessage: "",
    clientRequestId: "",
    timezone: "Asia/Singapore",
    timeCaptured: false,
    entryMethod: "manual",
    recording: false,
  },

  onLoad() {
    this.environment = config.getActiveEnvironment();
    this.api = createApiClient(this.environment);
    this.recorder = wx.getRecorderManager();
    this.bindRecorderHandlers();
    this.startDraft();
  },

  bindRecorderHandlers() {
    if (this.recorderHandlersBound) return;
    this.recorderHandlersBound = true;
    this.recorder.onStop(async (result) => {
      this.setData({recording: false});
      if (!result.tempFilePath) {
        this.setData({error: "未获取到录音，可以继续手动输入。"});
        return;
      }
      let fileSize = result.fileSize;
      if (!fileSize && wx.getFileInfo) {
        try {
          const info = await wx.getFileInfo({filePath: result.tempFilePath});
          fileSize = info.size;
        } catch (error) {
          fileSize = 0;
        }
      }
      await this.parseTemporaryFile({path: result.tempFilePath, purpose: "voice", extension: "mp3", contentType: "audio/mp3", sizeBytes: fileSize, durationSeconds: Math.ceil((result.duration || 0) / 1000)});
    });
    this.recorder.onError(() => {
      this.setData({recording: false, error: "录音失败，可以继续手动输入。"});
    });
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
      entryMethod: "manual",
      error: "",
      assistMessage: "",
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

  applyCandidate(candidate, method) {
    const patch = {
      entryMethod: method,
      status: "draft_ready",
      assistMessage: "识别结果已填入草稿，请核对后再保存。无法确定的字段已留空。",
      error: "",
      systolic1: "",
      diastolic1: "",
      pulse1: "",
    };
    if (candidate.systolic && candidate.systolic.value !== null) patch.systolic1 = String(candidate.systolic.value);
    if (candidate.diastolic && candidate.diastolic.value !== null) patch.diastolic1 = String(candidate.diastolic.value);
    if (candidate.pulse && candidate.pulse.value !== null) patch.pulse1 = String(candidate.pulse.value);
    this.setData(patch);
  },

  async startVoice() {
    this.captureFreshTimeIfNeeded();
    this.setData({assistMessage: "请说：高压一百三十二，低压八十四，心率七十。再次点击停止。", error: ""});
    try {
      await wx.authorize({scope: "scope.record"});
    } catch (error) {
      this.setData({error: "麦克风权限未开启，可以继续手动输入。"});
      return;
    }
    this.setData({recording: true, status: "recording"});
    this.recorder.start({duration: 15000, format: "mp3", numberOfChannels: 1});
  },

  stopVoice() {
    if (this.recorder) this.recorder.stop();
  },

  async choosePhoto() {
    this.captureFreshTimeIfNeeded();
    this.setData({assistMessage: "请拍清血压计屏幕，尽量包含 SYS/DIA/PUL 标签。", error: ""});
    try {
      const result = await wx.chooseMedia({count: 1, mediaType: ["image"], sourceType: ["camera", "album"], sizeType: ["compressed"]});
      const file = result.tempFiles && result.tempFiles[0];
      if (!file) {
        this.setData({error: "未选择照片，可以继续手动输入。"});
        return;
      }
      await this.parseTemporaryFile({path: file.tempFilePath, purpose: "photo", extension: "jpg", contentType: "image/jpeg", sizeBytes: file.size});
    } catch (error) {
      this.setData({error: "未完成拍照或相册选择，可以继续手动输入。"});
    }
  },

  async parseTemporaryFile({path, purpose, extension, contentType, sizeBytes, durationSeconds}) {
    const cloudPath = randomCloudPath(purpose, extension);
    let fileID = "";
    this.setData({status: "uploading", error: ""});
    try {
      const upload = await wx.cloud.uploadFile({cloudPath, filePath: path});
      fileID = upload.fileID;
      this.setData({status: "parse"});
      const parsed = await wx.cloud.callFunction({
        name: this.environment.mediaParserFunctionName,
        data: {
          purpose,
          file: {fileID, contentType, sizeBytes, durationSeconds},
        },
      });
      const result = parsed.result || {};
      if (result.candidate) {
        this.applyCandidate(result.candidate, purpose === "voice" ? "voice" : "photo");
      } else if (result.recognizedText) {
        const interpreted = await this.api.interpretBPEntry({recognizedText: result.recognizedText});
        this.applyCandidate(interpreted.candidate, purpose === "voice" ? "voice" : "photo");
      } else {
        this.setData({status: "error", error: "没有识别到明确读数，可以继续手动输入。"});
      }
    } catch (error) {
      this.setData({status: "error", error: "识别失败，可以继续手动输入。"});
    } finally {
      if (fileID) {
        try {
          await wx.cloud.deleteFile({fileList: [fileID]});
        } catch (error) {
          this.setData({assistMessage: "临时文件清理稍后重试；请继续核对草稿。"});
        }
      }
    }
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
        entryMethod: this.data.entryMethod,
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
