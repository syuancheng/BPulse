# Environment variable matrix

| Variable | Local | Test | Production | Secret |
|---|---|---|---|---|
| `APP_ENV` | `local` | `test` | `production` | No |
| `PORT` | `8080` | platform | platform | No |
| `MYSQL_DSN` | local Docker | test DB | production DB | Yes |
| `MIGRATION_STATEMENT_TIMEOUT` | `5m` | operator configured | operator configured | No |
| `DATA_ENCRYPTION_KEY_B64` | synthetic 32-byte key when needed | isolated test key | isolated production key | Yes |
| `DATA_ENCRYPTION_KEY_VERSION` | `v1`/test label | rotation label | rotation label | No |
| `CLOUDBASE_ENV_ID` | optional | test environment | production environment | No |
| `CLOUDBASE_SERVICE_NAME` | optional | test service | production service | No |
| `WECHAT_APP_ID` | unset/fake | test AppID | production AppID | Usually no |
| `WECHAT_APP_SECRET` | unset | secret store | secret store | Yes |
| `WECHAT_TEMPLATE_ID` | unset/fake | test template | production template | No |
| `NOTIFIER_MODE` | `fake` | `fake` until manual validation | explicit `real` | No |
| `MEDIA_PROVIDER_MODE` | `fake` | `fake` in ordinary CI/E2E | explicit `real` | No |
| `OCR_REGION` / `ASR_REGION` | unset | configured if manually tested | configured | No |
| `LOG_LEVEL` | `debug`/`info` | `info` | `info` | No |

ASR/OCR provider credentials are function-side workload identity or secret-store
values and are deliberately absent from `.env.example`. Test and production must
not share databases, encryption keys, logs, services, or workers.

Identity header policy is environment-bound and not configurable by clients:

`APP_ENV` is required; there is no implicit local fallback. MySQL DSNs are
normalized at startup to force `parseTime=true`, Go `UTC`, and MySQL session
`time_zone='+00:00'` on every pooled connection.

| Environment | Accepted identity | Explicitly rejected |
|---|---|---|
| `local` | `X-Debug-OpenID` with synthetic value | `X-WX-OPENID` alone |
| `test` | trusted CloudBase `X-WX-OPENID` | any `X-Debug-OpenID` |
| `production` | trusted CloudBase `X-WX-OPENID` | any `X-Debug-OpenID` |

The deployment must remain mini-program-only through CloudBase Run so public
clients cannot supply a header that is trusted only because CloudBase injected it.
