# Quick Start：从零到微信小程序上线

## 0. 最终交付目标

完成以下闭环后才算“上线”：

1. 用户通过微信进入小程序。
2. 用户能创建每日测血压、运动和饮食任务。
3. 用户进入记录页时自动获得当前测量时间，也能手动修改日期和时间。
4. 用户能通过手动、语音或拍照生成血压草稿，核对后保存，并查看最近趋势。
5. 用户主动订阅后，系统能发送一次提醒。
6. 用户能绑定家属并独立授权查看范围。
7. 测试环境通过验收。
8. 微信体验版通过真机测试。
9. 完成隐私指引、服务类目、审核与正式发布。

首版不包含自动诊断、治疗建议、自动调整药量、生成式 AI 风险预测和蓝牙血压计。语音和拍照仅辅助填表，必须人工确认。

---

## 1. 准备账号与本机工具

需要：

- 微信小程序账号和 AppID。
- 与小程序关联的腾讯云/CloudBase 账号。
- GitHub 仓库。
- Git、Docker Desktop、稳定版 Go、Node.js LTS。
- 微信开发者工具。
- Codex CLI。
- 已开通的腾讯云一句话语音识别和 OCR 服务（可以在完成手动录入后再开通）。

安装 Codex CLI 后，在终端执行：

```bash
codex
```

首次运行按提示登录。不要使用跳过沙箱或跳过审批的危险模式。

---

## 2. 创建仓库

```bash
mkdir bp-companion
cd bp-companion
git init
mkdir -p docs .github/workflows
```

将本实施包中的 `AGENTS.md`、`CODEX_PROMPTS.md`、`REVIEW_CHECKLIST.md` 和 PR 模板复制到仓库。

建议的最终结构：

```text
bp-companion/
├── AGENTS.md
├── README.md
├── Makefile
├── docker-compose.yml
├── .env.example
├── miniprogram/
├── server/
├── media-parser/
├── reminder-worker/
├── migrations/
├── scripts/
├── docs/
└── .github/
    ├── workflows/ci.yml
    └── PULL_REQUEST_TEMPLATE.md
```

---

## 3. 让 Codex 创建执行计划

在仓库根目录启动 Codex：

```bash
codex
```

先使用计划模式，然后复制 `CODEX_PROMPTS.md` 中的 **Phase 0**：

```text
/plan
```

要求 Codex 先输出：

- 目录结构；
- API 契约；
- 数据库 ERD；
- 里程碑；
- 风险清单；
- 验收标准。

计划经你检查后，再让它实现。不要直接用一句“把整个项目做完”。

---

## 4. 本地环境

Codex 应生成以下命令：

```bash
cp .env.example .env.local
docker compose up -d mysql
make migrate-up
make test
make run-api
```

本地环境必须满足：

- MySQL 在 Docker 中运行。
- Go API 使用 `APP_ENV=local`。
- 本地身份只接受 `X-Debug-OpenID`，非 local 环境必须禁用。
- 微信消息发送使用 `NOTIFIER_MODE=fake`。
- 语音/OCR 使用 `MEDIA_PROVIDER_MODE=fake`；普通 CI 不调用真实腾讯云接口。
- 测试数据全部是虚构数据。
- 日志不得出现 openid 全值、血压明文、备注、AppSecret 或数据库密码。

本地验收：

```bash
curl http://localhost:8080/healthz
curl -H 'X-Debug-OpenID: test-patient-001' \
  http://localhost:8080/api/v1/tasks/today
```

---

## 5. MVP 开发顺序

严格按以下顺序实现，每个阶段单独提交和评审：

1. 工程骨架、健康检查、迁移、CI。
2. CloudBase 身份头中间件与本地调试身份。
3. 今日任务和护理计划。
4. 当前时间默认值、可编辑测量时间、手动录入、加密记录与趋势。
5. 语音输入、拍照 OCR、统一确认草稿和临时文件清理。
6. 家属绑定和字段级授权。
7. 订阅消息授权记录与提醒 Worker。
8. 隐私页、账户删除、数据导出。
9. 可观测性、限流、重试和发布脚本。

每个阶段结束执行：

```bash
make verify
```

然后在 Codex CLI 中运行：

```text
/review
```

所有 P0/P1 问题解决后才能进入下一阶段。

---

## 6. 创建 CloudBase 测试环境

在 CloudBase 控制台创建独立测试环境，例如：

```text
bp-companion-test
```

测试环境需要：

- 一个 CloudBase Run 服务：`bp-api-test`。
- 一个独立 MySQL 数据库。
- 一个媒体解析云函数：`bp-media-parser-test`。
- 一个定时云函数：`bp-reminder-worker-test`。
- 日志和告警。

CloudBase Run 配置：

```text
APP_ENV=test
PORT=8080
MYSQL_DSN=<test database DSN>
DATA_ENCRYPTION_KEY_B64=<32-byte key, base64>
WECHAT_APP_ID=<AppID>
WECHAT_APP_SECRET=<secret>
WECHAT_TEMPLATE_ID=<test template id>
NOTIFIER_MODE=real
MEDIA_PROVIDER_MODE=fake
OCR_REGION=<region>
ASR_REGION=<region>
LOG_LEVEL=info
```

这些值只能配置在 CloudBase 环境变量/密钥中，不能提交到 Git。ASR/OCR 的 SecretId/SecretKey 只放在媒体解析函数的密钥配置中；优先使用最小权限角色或子账号，不得放入小程序。

CloudBase Run 只开放小程序访问。小程序使用 `wx.cloud.callContainer`，不要在第一版配置公开 API 域名。

---

## 7. 小程序连接测试环境

`app.js` 初始化云环境：

```javascript
App({
  onLaunch() {
    wx.cloud.init({ env: 'bp-companion-test' })
  }
})
```

统一封装 API：

```javascript
async function callApi({ path, method = 'GET', data }) {
  const response = await wx.cloud.callContainer({
    config: { env: 'bp-companion-test' },
    path,
    method,
    data,
    header: {
      'X-WX-SERVICE': 'bp-api-test',
      'content-type': 'application/json'
    }
  })

  if (response.statusCode < 200 || response.statusCode >= 300) {
    throw new Error(`API failed: ${response.statusCode}`)
  }
  return response.data
}
```

生产发布前，构建配置必须切换到生产环境，不能在业务代码中到处硬编码环境 ID。

---

## 8. 测试环境验收

使用微信开发者工具生成体验版，并至少用两台真实手机、两个微信账号测试：一个患者、一个家属。

必须通过：

- 首次进入自动建立用户档案。
- 创建、编辑、禁用每日任务。
- 同一天不会重复生成任务。
- 打开记录页默认显示当前本地时间；停留后开始录入时会刷新为新的当前时间。
- 可以手动修改测量日期和时间，保存后时区转换正确。
- 手动输入只需高压、低压，心率可选；第二次读数可选。
- 语音示例“高压一百三十二，低压八十四，心率七十”能生成可编辑草稿。
- 拍摄/选择血压计图片能生成可编辑草稿；模糊或冲突字段留空。
- 任何语音/OCR 结果都不能自动保存，必须点击“确认保存”。
- 拒绝麦克风/相机权限、取消、断网或供应商失败时仍可手动输入。
- 连续点击保存不会生成重复记录。
- 临时音频/图片在成功和失败路径都被删除，日志和数据库中没有文件ID、签名URL、转写文本或OCR文本。
- 血压记录加密存储，数据库中看不到明文读数。
- 患者 A 不能读取患者 B 的任何数据。
- 家属未获授权时看不到具体血压。
- 解除绑定后家属立即失去访问权限。
- 重复触发 Worker 不会重复发送同一提醒。
- 用户拒绝订阅时不反复弹窗。
- 删除账户后个人数据按设计清除或进入可审计的删除流程。
- 网络失败时页面可恢复，不重复提交。

定时 Worker 在测试环境先使用手动触发；验证完成后才启用低频 Cron。

---

## 9. 创建生产环境

创建第二个、完全独立的环境：

```text
bp-companion-prod
```

禁止测试和生产共用：

- 数据库；
- 加密密钥；
- 日志；
- Worker；
- CloudBase Run 服务；
- 环境变量。

部署顺序：

1. 创建数据库并备份。
2. 执行向后兼容迁移。
3. 部署 Go API。
4. 执行 `/healthz` 和数据库 smoke test。
5. 部署媒体解析函数，先使用 fake provider 完成 smoke test，再手动启用真实 ASR/OCR。
6. 部署提醒 Worker，但暂不启用定时器。
7. 构建指向生产环境的小程序体验版。
8. 完成最终验收。
9. 启用生产定时器。

---

## 10. 微信后台上线准备

提交审核前完成：

- 小程序名称、图标和简介。
- 与实际功能一致的服务类目；若后台要求资质，按要求补充，不能选择不匹配类目规避审核。
- 用户隐私保护指引，准确列出健康记录、openid、家属关系、麦克风、相机/相册、临时云存储、ASR/OCR 处理和用途。
- 隐私政策、用户协议、账户删除入口和联系方式。
- 订阅消息模板。
- 所有隐私接口的授权流程；在申请麦克风或相机权限前说明用途。
- 所有页面可访问，无测试按钮、假数据和调试入口。
- 产品文案明确说明：本产品用于记录和提醒，不替代医生诊断，不提供药量调整。

---

## 11. 提交审核与发布

1. 在微信开发者工具中切换生产配置。
2. 执行完整构建和 `make verify`。
3. 上传代码，填写清晰版本说明。
4. 在微信公众平台将该版本设为体验版，完成最后一次真机回归。
5. 提交审核。
6. 审核通过后手动发布。
7. 发布后立即执行生产 smoke test。
8. 观察错误率、提醒失败率和数据库连接情况。

首个生产版本建议只开放给少量家人测试，不立即公开推广。

---

## 12. 上线完成定义

以下项目全部为“是”才算完成：

```text
[ ] main 分支 CI 全绿
[ ] Codex /review 无未解决 P0/P1
[ ] 人工 Review Checklist 已签字
[ ] 测试环境 E2E 通过
[ ] 手动、语音、拍照和测量时间编辑真机测试通过
[ ] OCR/ASR 无自动保存且临时文件清理通过
[ ] ASR/OCR 限额、预算告警和密钥权限已核对
[ ] 生产数据库已备份
[ ] 生产密钥未进入 Git
[ ] 体验版真机回归通过
[ ] 隐私指引与实际代码一致
[ ] 微信审核通过并发布
[ ] 发布后 smoke test 通过
[ ] 回滚方案已验证
```
