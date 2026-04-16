# 🛡️ T-Guard | Autonomous AI Traffic Sentinel

**T-Guard** is a high-performance, precision AI gateway designed to protect your budget. It monitors, routes, and controls AI traffic (OpenAI, Claude, etc.) in real-time, ensuring you never exceed your spending limits.

---

## 🌟 Core Features
- **Budget Guardian**: Set hard/soft daily spending limits. Automatically cuts off traffic to prevent unexpected bills.
- **Smart Routing**: High-performance Aho-Corasick engine to route requests based on model names or custom rules.
- **Real-time Monitoring**: A beautiful Terminal UI (TUI) providing live cost tracking and request logs.
- **Process Orchestration**: Automatically manages child processes (like CLI tools) and injects necessary proxy configurations.
- **Enterprise Ready**: Includes SSE (Server-Sent Events) interception for accurate streaming token calculation and secure credential management.

---

## 🚀 Quick Start (3 Steps)

### 1. Configuration
Copy `config.example.yaml` to `config.yaml`.
Open it and fill in your API keys, upstream URLs, and daily budget limits.

### 2. One-Click Setup
Run the following command in your terminal:
```bash
# This will install dependencies, create config, and build the binary
./setup.sh
```

### 3. Launch the Sentinel
- **Dashboard Mode** (View real-time usage):
  ```bash
  ./t-guard
  ```
- **Proxy Mode** (Run a tool under T-Guard's protection):
  ```bash
  ./t-guard -- your-ai-command
  ```

---

## 🛠️ FAQ

**Q: Do I need to be a developer to use this?**
A: **No**. T-Guard is designed to be user-friendly. Just edit the `config.yaml` file, and you're good to go.

**Q: Does it support streaming (SSE)?**
A: **Yes**. T-Guard intercepts streaming responses (like ChatGPT's typing effect) to calculate tokens with millicent precision.

**Q: Is it secure?**
A: Yes. It supports `X-TGuard-Auth` token validation and uses system keyrings for sensitive credential storage.

---

## ⚖️ License
Built by Dennis
