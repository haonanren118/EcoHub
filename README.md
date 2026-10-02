<div align="center">

<img src="logo.png" alt="EcoHub" width="120" />

# EcoHub

[![Release](https://img.shields.io/github/v/release/haonanren118/EcoHub)](https://github.com/haonanren118/EcoHub/releases)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-16-black?logo=nextdotjs&logoColor=white)](https://nextjs.org/)
[![MySQL](https://img.shields.io/badge/MySQL-8-4479A1?logo=mysql&logoColor=white)](https://www.mysql.com/)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white)](https://redis.io/)
[![License](https://img.shields.io/github/license/haonanren118/EcoHub)](./LICENSE)

**自托管影视聚合系统**

中文 | [English](./docs/README_EN.md)

[部署指南](./docs/README-Deploy.md) · [常见问题](./docs/README-FAQ.md) · [交流群组](#交流社区)

</div>

> **使用须知**
> EcoHub 不提供、不存储任何影视文件。片源来自使用者自行配置的采集接口。请遵守所在地区的法律法规以及各源站的使用约定，由此产生的风险由使用者自行承担。本项目仅供学习与技术交流。

---

## 关于本仓库（发行版说明）

本仓库是 [fe-spark/EcoHub](https://github.com/fe-spark/EcoHub) 的**国内可用性增强发行版**，在保留上游全部功能的前提下，针对两个实际部署痛点做了最小侵入式修复，并补充了完整的两层分类树重建能力。

**本版相对上游的改动如下：**

| # | 文件 | 改动内容 | 解决的问题 |
| --- | --- | --- | --- |
| 1 | `Dockerfile` | server-builder 阶段新增 `ARG GOPROXY`（默认 `https://goproxy.cn,direct`） | 上游默认走 `proxy.golang.org`，境内构建必然超时失败（实测 455s 超时），加参数后 Go 依赖下载恢复至可接受耗时 |
| 2 | `Dockerfile` | web-deps 阶段新增 `ARG NPM_REGISTRY`（默认 `https://registry.npmmirror.com`） | 前端依赖安装走境外源缓慢，指定国内镜像后 `npm ci` 实测约 76s |

两处改动均为**向后兼容的构建期参数**：海外环境不传参即保持上游默认行为，境内构建可通过 `--build-arg` 覆盖任意自有代理。

```bash
# 境内构建示例
docker build \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg NPM_REGISTRY=https://registry.npmmirror.com \
  -t ecohub:latest .
```

> **关于分类层级**：上游主站分类同步要求"最多两层"，本版通过升级至已移除语义推断逻辑的新版采集内核，使**任意采集源均可作为主站**且分类树可正确重建（原理见下文「分类树说明」）。

### 分类树说明（技术背景）

EcoHub 的数据模型为 **1 主站（`grade=0`） + N 从站（`grade=1`）**，且分类树存在**两层硬约束** —— 这是贯穿 Category 表存储、缓存键、`match_key` 与前端导航的领域不变量，不是可调参数。

上游旧版使用 `InferCategoryParentsBySemantic()` 按**分类名称语义**猜测父级，会把 `综艺片` / `动漫片` / `AI漫剧` / `体育` 等一级分类错误挂载到 `电影片` 之下，导致同步时报 `分类层级最多支持两层`。

本版采用的采集内核已将上述逻辑替换为 `inferCategoryParents()`：改为拉取 `?ac=detail` 详情页读取真实 `TypeID1` 字段来推断父级；且仅在源站**全部分类 `Pid==0`（完全平铺）**时才触发推断（`needsCategoryParentInference()`）。13 个主流源实测结论：8 个源自带两层结构，5 个全平铺源经详情页推断后最大深度仍 ≤ 1，**全部可直接作为主站**。

## 简介

EcoHub 是一款高性能、现代化的全栈多源影视聚合系统。它不仅提供极致流畅的 Web 观影体验，还集成了强大的自动化采集与管理后台，旨在帮助开发者和影视爱好者快速搭建属于自己的私人影视库。

客户端是独立 App 仓库（本项目以 Git Submodule 形式接入在 `app-for-ohos/` 与 `app-for-android/`）：

- **EcoHub for OHOS** (鸿蒙客户端): [fe-spark/EcoHub-for-OHOS](https://github.com/fe-spark/EcoHub-for-OHOS)
- **EcoHub for Android** (安卓客户端): [fe-spark/EcoHub-for-Android](https://github.com/fe-spark/EcoHub-for-Android)（正在适配开发中）

## 快速开始

要求 Docker 20+、Compose 2+，建议配置不低于 2 核 2 GB 内存。

```bash
git clone https://github.com/haonanren118/EcoHub.git
cd EcoHub
```

编辑 `.env`：将 `openssl rand -hex 32` 的输出写入 `JWT_SECRET`，并修改 MySQL / Redis 密码，然后启动：

```bash
docker compose up -d
```

| 地址 | 说明 |
| --- | --- |
| `http://<主机>:3000` | 前台 |
| `http://<主机>:3000/manage` | 管理后台 |
| `http://<主机>:3000/api` | 客户端（[EcoHub for OHOS](https://github.com/fe-spark/EcoHub-for-OHOS) / [EcoHub for Android](https://github.com/fe-spark/EcoHub-for-Android)）服务接入地址 |
| `http://<主机>:3000/api/provide/config` | TVBox / 影视仓 订阅地址 |
| `http://<主机>:3000/api/provide/vod` | MacCMS 兼容接口 |

默认账号：`admin` / `admin`（读写）、`guest` / `guest`（只读）。对外部署前须立即修改默认密码。

> **安全与网络建议**：生产环境建议通过 Nginx / 1Panel 配置反向代理与 HTTPS（80/443）。如需**不对公网暴露裸端口**，请在 `compose.yml` 的 `ports` 中显式绑定 `127.0.0.1:`（如 `127.0.0.1:3000:3000`），避免 Docker 默认规则绕过系统防火墙直接暴露端口；若无播放器直连需求，可直接注释 `18080` 端口映射。

安装完成后前台无数据，属预期行为。须在管理后台 **采集中心** 完成全量采集并发布后，前台才会展示影片。1Panel、外部数据库与反向代理见 [部署指南](./docs/README-Deploy.md)。

Telegram 通知在管理后台 **系统设置 → 通知配置** 中填写。境内服务器访问 Telegram 时经常出现超时，可在 `.env` 中设置 `TG_PROXY=http://host.docker.internal:7890`。详见 [server/notify.md](./server/notify.md)。

## 本地开发

请先在本地启动 MySQL 8 与 Redis 7。后端与前端需两个终端，均从仓库根目录执行。

API：

```bash
cd server
cp .env.example .env
go run ./cmd/server
```

Web：

```bash
cd web
cp .env.example .env.local
npm install
npm run dev
```

前台 `http://127.0.0.1:3000`，后台 `/manage`，API `http://127.0.0.1:8080`。[服务端](./server/README.md) · [前端](./web/README.md)

## 文档

| 文档 | 内容 |
| --- | --- |
| [部署指南](./docs/README-Deploy.md) | 安装脚本、1Panel、手动部署、反向代理与升级 |
| [常见问题](./docs/README-FAQ.md) | 空站、采集、缓存、登录 |
| [版本说明](./docs/RELEASE.md) | 变更记录、镜像 tag |
| [服务端](./server/README.md) / [前端](./web/README.md) | 环境变量、接口、本地启动 |
| [English](./docs/README_EN.md) | English overview |

## 交流社区

- **QQ 交流群：`708144970`** —— 本发行版部署答疑、源站配置交流、问题反馈
- 上游 Telegram 交流群：[https://t.me/ecohub_club](https://t.me/ecohub_club)

## 鸣谢

- **原作者 / 上游项目**：[fe-spark/EcoHub](https://github.com/fe-spark/EcoHub) —— 本发行版全部核心功能均来自上游，感谢原作者的开源与持续维护。若本版修复有价值，请优先向上游提交 PR / Issue。
- 上游在线演示与文档：见原仓库 README。

---

[PolyForm Noncommercial 1.0.0](./LICENSE) · [haonanren118/EcoHub](https://github.com/haonanren118/EcoHub) · [Issues](https://github.com/haonanren118/EcoHub/issues)
