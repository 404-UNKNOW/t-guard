# 项目上下文

## 项目是什么
T-Guard：一个高性能、毫美分精度的 AI 流量守卫网关，集成了实时路由、计费熔断、跨平台进程管理及 TUI 监控。

## 技术栈
- 语言：Go 1.26
- 界面：Bubbletea (TUI)
- 匹配：Aho-Corasick
- 存储：SQLite WAL
- 代理：httputil.ReverseProxy
- 进程管理：Unix PGID / Windows Job Object

## 核心架构
- **原子模块**：Token 计算、路由决策、原子计费。
- **集成层**：SSE 拦截代理、双缓冲监控 UI、子进程生命周期管理。

## 已交付功能
- [x] 双轨制 Token 计算引擎
- [x] SQLite WAL 异步数据层
- [x] Aho-Corasick 高性能路由引擎
- [x] 毫美分精度计费与流式熔断系统
- [x] SSE 拦截反向代理
- [x] 跨平台进程管理（环境变量注入、信号转发、安全清理）
- [x] 响应式终端监控 UI

## 运行指引
1. `go mod tidy` 安装依赖
2. `go build -o t-guard main.go` 编译
3. `./t-guard` 启动

## 变更记录
- 2026-04-15 完成 M0-M4 模块开发及全系统 main.go 集成。
- 2026-04-15 实现跨平台进程生命周期管理模块。
- 2026-04-16 进入稳定模式重构：
    - 统一依赖注入容器至 `internal/app`，实现主程序与 CLI 逻辑同步。
    - 增强配置校验系统，支持基于 Viper 的全量配置加载与验证。
    - 提升代理层健壮性，增加 Panic 恢复与 502/503 精准错误响应。
    - 优化存储层性能，实现 `model_stats` JSON 实时统计。
- 2026-04-16 启动生产化改造 (P0):
    - 实现定价引擎 `pkg/pricing`，支持按模型分阶段（输入/输出）计费。
    - 重构 SSE 拦截器，实现基于 JSON 解析的实时 Token 计费与流式中途熔断。
    - 统一安全准入校验，子进程自动注入 `X-TGuard-Auth` 令牌。
- 2026-04-16 生产化进阶 (P1):
    - **多维度路由**：支持基于 Source IP 和自定义 HTTP Headers 的智能分流策略。
    - **多模态计费**：引入 Vision 模型图像输入计费模型（支持 Low/High Detail 自动解析）。
    - **Admin API**：实现 `/admin/rules` 接口，支持无需重启即可热加载路由规则。
    - **观测加固**：暴露 `/metrics` 接口（Prometheus 格式），实时上报预算与请求指标。
- 2026-04-16 合规与容错强化 (P1):
    - **容错降级 (Fallback)**：实现自动故障转移逻辑。当主上游响应失败时，自动重试并切换至备选上游。
    - **安全审计**：对 Admin API 的所有变更操作引入结构化审计日志记录。
    - **云原生交付**：提供多阶段构建的 `Dockerfile` 及基础 `Helm Chart` 结构。
