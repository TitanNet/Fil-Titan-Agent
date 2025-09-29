# Fil-Titan Agent

[English](README.md) | [中文](README_zh.md)

[![Go Version](https://img.shields.io/badge/Go-1.22.5+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![Community](https://img.shields.io/badge/Join-Community-blue)](https://discord.gg/your-discord-link) 

**`FIL-Titan-Agent` is an open-source application for Filecoin Storage Providers (SPs) that securely connects your underutilized bandwidth to the Titan Network, creating a new, subsidy-free revenue stream independent of block rewards.**

This repository contains the official source code for the FIL-Titan-Agent. By running this lightweight agent, you can monetize your idle infrastructure, turning underutilized resources into a stable and tangible commercial revenue stream.

### Core Features & Benefits

* ✅ **Generate Significant Additional Revenue**: Monetize your idle bandwidth and earn a stable cash flow independent of any subsidies. Our market-validated pricing is highly competitive:
    * Mainland China: **$350 / Gbps / month**
    * Key overseas markets (Brazil, Japan, Thailand, Vietnam): **$160 / Gbps / month**

* ✅ **Serve Top-Tier Commercial Clients**: Your resources will power professional PCDN (Peer-to-Peer Content Delivery Network) services for premier global content platforms, including **TikTok, Tencent, Bilibili, Kuaishou, and iQiyi**.

* ✅ **Effortless Deployment & Seamless Integration**: Get started with a simple deployment of our lightweight agent. It's designed to run in parallel with your existing Lotus/Venus mining operations without interfering with your sealing or storage tasks.

* ✅ **On-Chain Value Settlement**: All revenue from commercial activities is settled automatically via smart contracts in various currencies, including `FIL` or `USDFC`. This ensures that value is circulated and captured directly on the Filecoin blockchain.

* ✅ **Secure & Open-Source**: The codebase is fully open-source and transparent, allowing you to audit every line of code for security. The agent runs in an isolated environment and does not access any of your sensitive data.

---

## Quick Start

To get started with Fil-Titan Agent, please refer to our running tutorials and installation guides.

### Dependencies

- **multipass** (windows, macos, linux)
- **virtualbox** (windows)

📖 **Detailed Running Tutorials:** [Multi-Device Type Running Tutorials](https://titannet.gitbook.io/titan-network-cn/4-ce-jia-li-le-ce-shi-wang/titan-agent-an-zhuang-jiao-cheng)

### 1. Preparation

- **Download Titan Agent installation package (using linux/amd64 as an example):**
  - [agent-linux.zip](https://pcdn.titannet.io/test4/latest/agent-linux.zip)

### 2. Get Your Identity Key

- **Follow this tutorial to get your key:**
  - [Titan Network Key Acquisition Tutorial](https://titannet.gitbook.io/titan-network-cn/4-ce-jia-li-le-ce-shi-wang/ru-he-huo-qu-key)

### 3. Installation and Execution

#### A. Extract and Prepare

```bash
# Create installation directory
mkdir -p /data/.titannet

# Copy the installation package to the target directory
cp /path/to/your/agent-linux.zip /data/.titannet/

# Navigate to the directory and unzip the package
cd /data/.titannet
unzip agent-linux.zip
````

#### B. Set Permissions

```bash
chmod +x /data/.titannet/agent
```

#### C. Start Titan Agent

Replace `YOUR_KEY_HERE` with the actual key you obtained in Step 2.

```bash
nohup /data/.titannet/agent --working-dir=/data/.titannet \
  --server-url=[https://test4-api.titannet.io](https://test4-api.titannet.io) \
  --key=YOUR_KEY_HERE > /dev/null 2>&1 &
```

#### D. Verify the Process is Running

```bash
ps -ef | grep agent
```

#### E. Stop the Agent (If Needed)

```bash
# Find the process ID (PID)
ps -ef | grep agent

# Stop the process using its PID
kill <PID>
```

### 4\. Configure Auto-start on Boot (Using systemd)

This is recommended for production environments to ensure the agent restarts automatically.

#### A. Create systemd Service File

Remember to replace `YOUR_KEY_HERE` with your actual key.

```bash
sudo tee /etc/systemd/system/titan-agent.service <<EOF
[Unit]
Description=Titan Network Agent
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/data/.titannet
ExecStart=/data/.titannet/agent \\
    --working-dir=/data/.titannet \\
    --server-url=[https://test4-api.titannet.io](https://test4-api.titannet.io) \\
    --key=YOUR_KEY_HERE
Restart=always
RestartSec=30
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=titan-agent

[Install]
WantedBy=multi-user.target
EOF
```

#### B. Set Permissions and Enable the Service

```bash
sudo chmod 644 /etc/systemd/system/titan-agent.service
sudo systemctl daemon-reload
sudo systemctl enable --now titan-agent
```

#### C. Manage the Service

You can now manage the agent using standard `systemctl` commands:

  - **Start service:** `sudo systemctl start titan-agent`
  - **Stop service:** `sudo systemctl stop titan-agent`
  - **Restart service:** `sudo systemctl restart titan-agent`
  - **Check status:** `sudo systemctl status titan-agent`
  - **View logs:** `sudo journalctl -u titan-agent -f`

## License

This project is licensed under the MIT License - see the [LICENSE](https://www.google.com/search?q=LICENSE) file for details.

```
```
