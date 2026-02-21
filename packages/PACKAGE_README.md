# Liaison 安装包说明

## 打包内容

运行 `make package` 会生成一个完整的安装包，包含以下内容：

```
liaison-{VERSION}-linux-amd64/
├── bin/
│   ├── liaison              # 主服务二进制（Linux amd64）
│   └── liaison-edge         # Edge 连接器（Linux amd64）
├── edge/                    # 所有平台的 Edge 连接器
│   ├── liaison-edge-linux-amd64
│   ├── liaison-edge-linux-arm64
│   ├── liaison-edge-darwin-amd64
│   ├── liaison-edge-darwin-arm64
│   └── liaison-edge-windows-amd64.exe
├── web/                     # 前端静态文件（web/dist 的内容）
│   ├── index.html
│   ├── assets/
│   └── ...
├── etc/                     # 配置文件模板
│   ├── liaison.yaml
│   └── liaison-edge.yaml
├── systemd/                 # systemd 服务文件
│   ├── liaison.service
│   ├── install.sh
│   ├── uninstall.sh
│   └── README.md
├── VERSION                  # 版本号
└── README.md                # 项目说明
```

## 使用方法

### 1. 解压安装包

```bash
tar -xzf liaison-{VERSION}-linux-amd64.tar.gz
cd liaison-{VERSION}-linux-amd64
```

### 2. 安装服务

```bash
sudo ./install.sh
```

### 3. 配置前端文件路径

编辑 `/opt/liaison/conf/liaison.yaml`，添加前端文件目录配置：

```yaml
manager:
  listen:
    addr: 0.0.0.0:8080
    network: tcp
  db: /opt/liaison/data/liaison.db
  web_dir: /opt/liaison/web  # 前端文件目录
```

### 4. 复制前端文件

```bash
sudo cp -r web/* /opt/liaison/web/
sudo chown -R liaison:liaison /opt/liaison/web
```

### 5. 启动服务

```bash
sudo systemctl start liaison
sudo systemctl status liaison
```

## Edge 连接器部署

安装包中包含了所有平台的 Edge 连接器，可以根据目标系统选择合适的版本：

- **Linux x86_64**: `edge/liaison-edge-linux-amd64`
- **Linux ARM64**: `edge/liaison-edge-linux-arm64`
- **macOS Intel**: `edge/liaison-edge-darwin-amd64`
- **macOS Apple Silicon**: `edge/liaison-edge-darwin-arm64`
- **Windows x64**: `edge/liaison-edge-windows-amd64.exe`

部署到目标系统后，重命名为 `liaison-edge` 并配置相应的配置文件即可使用。

## 注意事项

1. **前端文件**: 前端文件需要复制到配置文件中指定的 `web_dir` 目录
2. **权限**: 确保 `liaison` 用户对相关目录有读写权限
3. **端口**: 如果使用 443 端口，确保 systemd service 已配置 `CAP_NET_BIND_SERVICE` capability
4. **TLS 证书**: 如果启用 TLS，需要配置证书文件路径
