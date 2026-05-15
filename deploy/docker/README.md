# Liaison Docker Compose 部署

把 server 端(`liaison` 管理面 + `frontier` 连接器网关 + `guacd` WebDesktop sidecar)跑在 Docker 里。Edge(连接器)依旧按原生方式安装到目标主机,不在本方案内。

## 前置

- Docker 20.10+ 和 `docker compose` 插件
- 在项目根目录先完成一次本地构建,产出二进制 / 前端 / edge 安装包:
  ```bash
  make package
  ```
  完成后会生成:
  - `bin/liaison`、`bin/frontier`、`bin/password-generator`
  - `web/dist/*`
  - `packages/edge/liaison-edge-*.tar.gz`
  - `dist/edge/install.sh` 等脚本

## 首次部署

```bash
cd deploy/docker
cp .env.example .env
# 编辑 .env,把 LIAISON_PUBLIC_HOST 改成你的公网 IP 或域名
vim .env

# 构建镜像
docker compose build

# 启动
docker compose up -d
```

首次启动会自动:
1. 生成自签 TLS 证书写入 `./certs/`
2. 渲染 `./conf/liaison.yaml`(实际在容器 volume 里)
3. 创建管理员账号 `LIAISON_ADMIN_EMAIL`,**随机密码打印到日志里**

拿初始密码:
```bash
docker compose logs liaison | grep -A5 "first-run credentials"
```

输出类似:
```
============================================================
  Liaison first-run credentials (shown ONCE, save them now)
  Email:    default@liaison.com
  Password: AbCd1234EfGh5678
  URL:      https://your-public-ip:443
============================================================
```

记下密码 —— 日志轮转后就看不到了。用这个密码登录 Web 控制台,之后在"设置"里改密码即可。

## 离线分发包

想在没有构建环境的机器上部署,或者把镜像带到内网?从仓库根跑:

```bash
make package-docker
```

产出 `liaison-<VERSION>-docker-amd64.tar.gz`(含 `docker save` 出来的 liaison + frontier + guacd 镜像、compose 文件、`.env.example`、`load.sh`)。用户解压后执行 `./load.sh` 即可加载镜像并启动服务。

## 数据持久化

`deploy/docker/` 下会生成三个目录,首次启动自动创建,**删除它们等于删库**:

| 目录 | 内容 |
|:---|:---|
| `data/` | SQLite 数据库 `liaison.db`、初始化标记 |
| `certs/` | `server.crt` + `server.key`(liaison/frontier 共享) |
| `logs/` | liaison 进程日志 |

`.gitignore` 已经忽略这三个目录。

## 网络与端口

liaison / frontier / guacd 容器以 `network_mode: host` 运行 —— 不走 docker bridge,也不做公网端口映射。`MANAGER_PORT` / `FRONTIER_PORT` 是直接绑在宿主机网卡上的端口,跟 systemd 部署完全等价。liaison 容器加了 `cap_add: NET_BIND_SERVICE`,所以可以以 uid=1000 的非 root 用户绑 443 这种特权端口。guacd 显式绑定宿主机 `127.0.0.1:4822`，不对公网开放。

| 端口 | 用途 | 是否对外 |
|:---|:---|:---|
| `MANAGER_PORT`(默认 443) | Web 控制台 HTTPS | 是 |
| `FRONTIER_PORT`(默认 30012) | 连接器接入 | 是 |
| `FRONTIER_CONTROLPLANE_PORT`(默认 30010) | liaison 主动断开连接器连接 | 否(loopback) |
| 127.0.0.1:30011 | liaison ↔ frontier 本地通信 | 否(loopback) |
| 127.0.0.1:4822 | liaison ↔ guacd WebDesktop 转换 | 否(loopback) |

Edge 连接器在 Web 里创建时会自动把 `LIAISON_PUBLIC_HOST:FRONTIER_PORT` 写进安装命令。改了 `FRONTIER_PORT` 之后,已发出去的 edge 安装命令也会指向新端口,老的 edge 需要重新下发。

WebDesktop(RDP/VNC) 依赖 `guacd` sidecar。Compose 默认启动 `guacamole/guacd:1.5.5`，绑定 `127.0.0.1:4822`，并让 liaison 连接这个本机地址。guacd 回连 manager 创建的临时桥接端口也走 `127.0.0.1`，所以临时端口不会暴露到公网。

## 常用操作

```bash
# 查看状态
docker compose ps

# 跟踪日志
docker compose logs -f liaison
docker compose logs -f frontier
docker compose logs -f guacd

# 升级:重新 make package,然后
docker compose build --no-cache
docker compose up -d

# 彻底删除(保留数据)— 对应离线包里的 ./uninstall.sh
docker compose down
docker rmi liaison/liaison:1.8.0 liaison/frontier:1.8.0 guacamole/guacd:1.5.5

# 彻底删除并清空数据(不可恢复)— 对应离线包里的 ./uninstall.sh --purge
docker compose down
docker rmi liaison/liaison:1.8.0 liaison/frontier:1.8.0 guacamole/guacd:1.5.5
rm -rf data certs logs .env
```

离线分发包(`make package-docker` 产物)附带 `uninstall.sh` 做同样的事,方便不记命令:

```bash
./uninstall.sh            # 停容器 + 删镜像,保留数据
./uninstall.sh --purge    # 再删 data/certs/logs/.env,需键入 yes 确认
```

## 重置管理员密码

```bash
docker compose exec liaison /opt/liaison/bin/password-generator \
    -password "NewPassword123" \
    -email "default@liaison.com"
```

不加 `-create`,只改现有用户密码。

## 常见问题

**Q: 浏览器提示证书不受信任?**
A: 方案里用的是自签证书。把 `certs/server.crt` 导入系统信任即可,或者换成你自己的证书 —— 直接替换 `certs/server.crt` 和 `certs/server.key` 后 `docker compose restart`。

**Q: 改 `LIAISON_PUBLIC_HOST` 之后需要重新生成证书吗?**
A: 是的。删掉 `certs/server.*` 然后 `docker compose restart liaison`,entrypoint 会重新生成。

**Q: 能换成 nginx 前置?**
A: 可以,把 `MANAGER_PORT` 绑回 `127.0.0.1:8443`,前面架 nginx 反代到 `https://127.0.0.1:8443`。`server_url` 通过 `.env` 的 `SERVER_URL` 手动指定(取消 compose 文件里对应注释)。
