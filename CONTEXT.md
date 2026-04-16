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
