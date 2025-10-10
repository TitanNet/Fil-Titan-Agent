# Fil-Titan Agent
[English](README.md) | [中文](README_zh.md)

[![Go Version](https://img.shields.io/badge/Go-1.22.5+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)

**`FIL-Titan-Agent` 是一个为 Filecoin 存储提供商 (SP) 设计的开源软件，它能安全地将您存储设施中未被充分利用的上行带宽资源，接入到 Titan 的商业服务网络中，为您创造一个独立于区块奖励的全新、无需补贴的收入来源。**

本仓库包含了 `FIL-Titan-Agent` 的官方源代码。通过运行这个轻量级代理，您可以将闲置的基础设施转化为稳定、真实的商业收益。

### 核心价值与优势

* ✅ **创造可观的额外收入**: 将闲置带宽变现，获得不依赖于任何补贴的稳定现金流。我们的商业模式已经过市场验证，定价极具吸引力：
    * 中国大陆市场: **$350 / Gbps / 月**
    * 巴西、日本、泰国、越南等海外重点市场: **$160 / Gbps / 月**

* ✅ **服务顶级商业客户**: 您的资源将为全球顶级内容平台提供专业的 PCDN (点对点内容分发网络) 服务，客户包括 **TikTok、腾讯、Bilibili、快手、爱奇艺** 等。

* ✅ **一键部署，无缝集成**: 只需一键部署我们的轻量级 Agent，即可轻松加入网络。它被设计为与您现有的 Lotus/Venus 挖矿业务并行运行，互不干扰，不影响您当前的封装和存储任务。

* ✅ **价值链上结算**: 所有商业活动的收益将通过智能合约以 `FIL` 或 `USDFC` 等多种形式进行自动化结算，确保价值直接在 Filecoin 链上流通和沉淀。

* ✅ **安全且开源**: 代码完全开源透明，您可以审查每一行代码，确保其安全性。Agent 运行在隔离环境中，不会访问您的任何敏感数据。

---

## 工作原理

下图展示了 Titan Network 完整的价值闭环。它清晰地描绘了终端用户的付款如何流经整个系统，最终用于奖励贡献了带宽资源（阶段一）以及未来贡献热存储资源（阶段二）的存储提供商。
 <img width="1540" height="1137" alt="image" src="https://github.com/user-attachments/assets/1d5c6fb7-6d4d-451c-a297-725e81b04c07" />


---

## 快速开始

要开始使用 Fil-Titan Agent，请参阅我们的运行教程和安装指南。

### 依赖

- **multipass** (windows, macos, linux)
- **virtualbox** (windows)

📖 **详细运行教程：** [各设备类型运行教程](https://titannet.gitbook.io/titan-network-cn/4-ce-jia-li-le-ce-shi-wang/titan-agent-an-zhuang-jiao-cheng)

### 1. 准备工作

- **下载 Titan Agent 安装包 (以 linux/amd64 为例):**
  - [agent-linux.zip](https://pcdn.titannet.io/test4/latest/agent-linux.zip)

### 2. 获取您的身份密钥 (Key)

- **请参考此教程获取您的 Key:**
  - [Titan Network Key 获取教程](https://titannet.gitbook.io/titan-network-cn/4-ce-jia-li-le-ce-shi-wang/ru-he-huo-qu-key)

### 3. 安装与执行

#### A. 解压与准备

```bash
# 创建安装目录
mkdir -p /data/.titannet

# 复制安装包到目标目录
cp /您下载的路径/agent-linux.zip /data/.titannet/

# 进入目录并解压
cd /data/.titannet
unzip agent-linux.zip
````

#### B. 设置权限

```bash
chmod +x /data/.titannet/agent
```

#### C. 启动 Titan Agent

请将 `你的KEY` 替换为您在第2步中获取的实际 Key。

```bash
nohup /data/.titannet/agent --working-dir=/data/.titannet \
  --server-url=[https://test4-api.titannet.io](https://test4-api.titannet.io) \
  --key=你的KEY > /dev/null 2>&1 &
```

#### D. 验证进程是否在运行

```bash
ps -ef | grep agent
```

#### E. 停止 Agent (如果需要)

```bash
# 查找进程 ID (PID)
ps -ef | grep agent

# 使用 PID 停止进程
kill <PID>
```

### 4\. 配置开机自启动 (使用 systemd)

推荐在生产环境中使用，以确保 Agent 能自动重启。

#### A. 创建 systemd 服务文件

同样，请记得将 `你的KEY` 替换为您的实际 Key。

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
    --key=你的KEY
Restart=always
RestartSec=30
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=titan-agent

[Install]
WantedBy=multi-user.target
EOF
```

#### B. 设置权限并启用服务

```bash
sudo chmod 644 /etc/systemd/system/titan-agent.service
sudo systemctl daemon-reload
sudo systemctl enable --now titan-agent
```

#### C. 管理服务

您现在可以使用标准的 `systemctl` 命令来管理 Agent:

  - **启动服务:** `sudo systemctl start titan-agent`
  - **停止服务:** `sudo systemctl stop titan-agent`
  - **重启服务:** `sudo systemctl restart titan-agent`
  - **查看状态:** `sudo systemctl status titan-agent`
  - **查看日志:** `sudo journalctl -u titan-agent -f`

## 许可证 (License)

本项目基于 MIT 许可证 - 详情请参阅 [LICENSE](LICENSE) 文件。

```
MIT License

Copyright (c) 2023 Filecoin Titan

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```