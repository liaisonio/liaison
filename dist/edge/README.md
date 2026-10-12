# Edge 安装脚本

## 目录结构

```
dist/edge/
├── install.sh      # 安装脚本（Linux/macOS，会根据操作系统自动下载对应的安装包）
├── install.ps1     # 安装脚本（Windows PowerShell，推荐）
├── install.bat     # 安装脚本（Windows 批处理脚本）
└── README.md       # 本文件
```

## 安装包目录

安装包需要放置在 `/opt/liaison/packages/edge/` 目录下，文件命名格式：

- Linux: `liaison-edge-linux-amd64.tar.gz` 或 `liaison-edge-linux-arm64.tar.gz`
- macOS: `liaison-edge-darwin-amd64.tar.gz` 或 `liaison-edge-darwin-arm64.tar.gz`
- Windows: `liaison-edge-windows-amd64.tar.gz`

## 安装脚本使用

安装脚本会：
1. 自动检测操作系统和架构
2. 从服务器下载对应的安装包（tar.gz 格式）
3. Linux/macOS 创建随机 ID 的独立实例；不覆盖旧实例
4. 创建 0600 配置文件和独立系统服务或用户服务

### 使用方法

#### Linux/macOS

```bash
# 新建独立实例；普通用户不会自动提权
curl -fsSL https://your-server/install.sh | bash -s -- \
  --new-instance \
  --access-key=YOUR_ACCESS_KEY \
  --secret-key=YOUR_SECRET_KEY \
  --server-http-addr=your-server:443 \
  --server-edge-addr=your-server:30012

# 升级选定实例，不重新提供或覆盖密钥
curl -fsSL https://your-server/install.sh | bash -s -- \
  --upgrade-instance=INSTANCE_ID \
  --server-http-addr=your-server:443
```

旧标准安装使用 `--upgrade-instance=legacy`。必须明确选择新建或升级，旧命令省略模式时会安全退出，不覆盖安装。重复使用同一 Manager/接入密钥创建实例也会被拒绝。

- Linux root：程序在 `/opt/liaison/edges/<id>/`，配置在 `/etc/liaison/edges/<id>/`，使用系统级 systemd。
- Linux 普通用户：位于 `~/.local/share/liaison/edges/<id>/`，使用 `systemctl --user`。需要可用的用户服务会话；不自动配置 linger。
- macOS：位于 `~/Library/Application Support/liaison/edges/<id>/`，使用独立 LaunchAgent；在对应登录用户下执行，不使用 sudo。

安装、升级仅确认服务进程启动，不代表已成功连接 Manager。升级保留配置和服务文件，启动失败尝试回滚；如果提示恢复文件或锁存在，请先检查，不要盲目重试。远程卸载默认允许，且受本地归属预检约束。

正常执行安装命令即可支持控制台卸载，无需额外参数。卸载仍要求管理员权限、名称确认及本机归属预检通过。需要禁止远程卸载时，可在本实例配置中显式设置 `allow_remote_uninstall: false` 并重启；升级会保留该设置。

#### Windows

独立 Edge 使用原生 Windows 服务，不依赖登录会话。以管理员身份打开 PowerShell 5.1 或更新版本；需要 x64 Windows、`curl.exe` 和 `tar.exe`。Liaison One 管理的前台 Edge 不使用此安装流程。

```powershell
# 下载脚本
Invoke-WebRequest -Uri 'https://your-server/install.ps1' -OutFile 'install.ps1'

# 运行脚本
.\install.ps1 -AccessKey YOUR_ACCESS_KEY -SecretKey YOUR_SECRET_KEY -ServerHttpAddr your-server:443 -ServerEdgeAddr your-server:30012
```

安装会新建 `LiaisonEdge-<实例 ID>` 服务及 `%ProgramFiles%\Liaison\edges\<实例 ID>` 目录，以 LocalService 运行，自动启动并配置失败重启。只有管理员和 SYSTEM 可以修改程序、配置；LocalService 可读取配置、写入日志。安装结束确认的是服务运行，仍需在平台确认连接器在线。

下载和连接均验证 TLS。私有 CA 使用 `-CACert C:\path\ca.pem`，不要关闭证书验证；下载脚本本身也应使用受信任的入口。遵守组织的脚本执行、签名与应用控制策略，本脚本不会绕过它们。

离线安装提供可信来源的校验值：

```powershell
.\install.ps1 -AccessKey YOUR_ACCESS_KEY -SecretKey YOUR_SECRET_KEY -ServerEdgeAddr your-server:30012 -PackagePath .\liaison-edge-windows-amd64.tar.gz -ExpectedSHA256 RELEASE_SHA256
```

`install.bat` 是同目录 `install.ps1` 的包装，使用相同 PowerShell 参数：

```cmd
install.bat -AccessKey YOUR_ACCESS_KEY -SecretKey YOUR_SECRET_KEY -ServerHttpAddr your-server:443 -ServerEdgeAddr your-server:30012
```

精确卸载需要安装时输出的实例 ID，可先使用 `-WhatIf`：

```powershell
.\uninstall.ps1 -InstanceId INSTANCE_ID -WhatIf
.\uninstall.ps1 -InstanceId INSTANCE_ID
```

`-KeepConfig` / `-KeepLogs` 可保留诊断文件。卸载会核验服务路径与账号，不按进程名称批量终止。旧版固定目录、其他实例和 One 不会自动迁移或删除。此阶段尚不支持 Windows 原地升级及远程卸载；不要重复安装同一连接器作为升级手段。Windows ARM64 尚未验证。

Git Bash 安装入口会安全退出，请改用 PowerShell。WSL 是独立 Linux 环境，使用 Linux 流程并启用 systemd，不安装 Windows 服务。

### 参数说明

- `--new-instance`: Linux/macOS 新建独立实例
- `--upgrade-instance`: Linux/macOS 指定实例 ID 或 `legacy`，与新建互斥
- `--access-key`: Access Key（新建必需）
- `--secret-key`: Secret Key（新建必需）
- `--server-http-addr`: 安装包下载 URL 或 HTTPS 的 `host:port`（必需）
- `--server-edge-addr`: Manager 的 Edge 连接地址 `host:port`（新建必需）
- `--help`: 显示帮助信息

## 配置说明

在 `liaison.yaml` 中配置：

```yaml
manager:
  server_url: "http://your-server:8080"  # 服务器 URL，用于生成安装命令（包含协议）
  packages_dir: "/opt/liaison/packages"   # 安装包目录（可选，默认 /opt/liaison/packages）
```

注意：发布时必须同步更新 Manager、`install.sh` 和 Edge 安装包。新脚本搭配不支持生命周期命令的旧二进制会失败，不应混合发布。

## API 端点

- `/install.sh` - 安装脚本下载
- `/packages/edge/{package-name}` - 安装包下载

这些端点不需要认证即可访问。
