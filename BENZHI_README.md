# BENZHI_README

基于 Go 实现的天文观测数据科研释放治理服务 HTTP API 项目，一款后端服务，已完整实现天文观测数据从归档建档、不可变修订登记、确定性校验、异常隔离和同行复核，到冻结 Merkle 清单、签发及验证科研释放凭据的单进程 Go 服务，并提供带校验链的本地持久化、乐观并发、幂等重放、审计时间线和真实 HTTP 自检。

## 项目说明
- 项目：benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57
- 项目用途：已完整实现天文观测数据从归档建档、不可变修订登记、确定性校验、异常隔离和同行复核，到冻结 Merkle 清单、签发及验证科研释放凭据的单进程 Go 服务，并提供带校验链的本地持久化、乐观并发、幂等重放、审计时间线和真实 HTTP 自检。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57-arm64 linux/arm64
docker run -it benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081`
