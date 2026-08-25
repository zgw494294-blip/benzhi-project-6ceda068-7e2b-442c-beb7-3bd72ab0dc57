# 天文观测数据科研释放治理服务

本项目面向小型天文台的数据管理员、观测数据质量审核员和科研释放负责人，提供单进程 Go JSON HTTP API。服务管理一次观测任务从建档、不可变数据集修订登记、确定性校验、异常隔离与处置复核，到冻结 Merkle 清单、签发科研释放凭据和重新验证凭据的完整流程。服务只保存文件元数据和 SHA-256 摘要，不接收大型观测文件本体。

## 构建、运行与测试

项目要求 Go 1.22 或更高版本，不依赖第三方 Go 模块。

```text
go build ./cmd/server
go run ./cmd/server
go test ./...
```

默认监听 `127.0.0.1:19081`，不会默认绑定 `0.0.0.0`。可用以下两种方式配置监听地址，其中显式 `-addr` 优先于 `PORT`：

```text
go run ./cmd/server -addr=127.0.0.1:19123
PORT=19123 go run ./cmd/server
```

监听地址必须是有效的回环地址。常规运行时数据默认保存在 `./data`，也可用 `-data-dir` 指定目录：

```text
go run ./cmd/server -addr=127.0.0.1:19123 -data-dir=./runtime-data
```

## 有界自检

自检模式会真实启动所配置的回环监听器，经 HTTP 依次完成任务创建、修订登记、校验、清单冻结、凭据签发和凭据验证，然后主动优雅关闭并明确退出。未指定 `-data-dir` 时，自检使用临时目录并在结束后清理。

```text
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
```

成功时日志包含 `SELF_CHECK_OK`，命令以状态码 `0` 退出；任一步骤失败都会返回非零状态。

## API 与业务约束

API 前缀为 `/api/v1`，健康检查为 `GET /healthz`。主要端点如下：

- `POST /api/v1/archive-tasks`：创建归档任务。
- `GET /api/v1/archive-tasks`：列出任务。
- `GET /api/v1/archive-tasks/{taskId}`：读取完整任务详情。
- `POST /api/v1/archive-tasks/{taskId}/revisions`：登记不可变数据集修订。
- `POST /api/v1/archive-tasks/{taskId}/validation`：执行确定性完整性与元数据校验。
- `POST /api/v1/archive-tasks/{taskId}/findings/{findingId}/resolution`：提交替代修订和处置说明。
- `POST /api/v1/archive-tasks/{taskId}/findings/{findingId}/review`：接受或退回处置。
- `POST /api/v1/archive-tasks/{taskId}/freeze`：冻结稳定排序的清单和 Merkle 根。
- `POST /api/v1/archive-tasks/{taskId}/release-credentials`：签发科研释放凭据。
- `GET /api/v1/archive-tasks/{taskId}/timeline`：读取完整审计时间线。
- `GET /api/v1/archive-tasks/{taskId}/release-verification`：重新计算清单和凭据摘要并验证绑定关系。

所有写请求使用 `application/json`，请求体最大为 1 MiB，未知 JSON 字段会被拒绝。每个变更请求都携带 `idempotencyKey`、`expectedVersion`、`actor`、`role`、`reason` 和 `correlationId`；创建操作的 `expectedVersion` 固定为 `0`。重复提交同一操作和 `idempotencyKey` 会返回首次结果，版本过期会返回机器可识别的 `VERSION_CONFLICT`。

角色标识固定为：

- `DATA_ADMIN`：创建任务、登记修订、执行校验和提交问题处置。
- `QUALITY_REVIEWER`：执行校验并接受或退回问题处置。
- `RELEASE_LEAD`：冻结清单并签发释放凭据。

任务状态按 `DRAFT`、`COLLECTING`、`QUARANTINED` 或 `REVIEW_PENDING`、`FROZEN`、`RELEASED` 推进。阻断问题必须关联直接替代问题修订的新修订；替代修订再次通过阻断规则检查且审核接受后，任务才可恢复到 `REVIEW_PENDING`。

## 本地持久化

数据目录包含 `events.jsonl` 和 `snapshot.json`。事件日志使用单调序号、前序哈希和当前记录哈希形成校验链，同时保存聚合投影、审计事件和幂等结果。每次提交先同步事件日志，再通过临时文件写入、文件 `Sync`、原子 `Rename` 和目录 `Sync` 替换带 `schemaVersion` 与校验和的投影快照。服务启动时验证完整日志链和快照，并重放快照之后的事件；截断、序号断裂或内容篡改会阻止启动。
