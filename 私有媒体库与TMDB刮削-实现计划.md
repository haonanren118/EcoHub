# 私有媒体库与 TMDB 刮削 — 实现计划

状态：待评审 / 待实现  
范围：后端系统运行模式 + WebDAV 存储挂载 + 文件名智能解析 + TMDB 刮削引擎 + 302 直链流播放 + 前端管理与菜单裁剪  
非目标：服务端实时视频转码（CPU密集型，坚持客户端直解）、第三方网盘私有反爬 SDK（统一由 Alist / WebDAV 承接）

---

## 1. 背景与目标

### 现状痛点
1. **片源渠道单一**：EcoHub 目前仅支持通过 MacCMS / JSON 采集站抓取影视切片，画质差（720P/假1080P）、常带跑马灯博彩水印、易失效死链；
2. **商业化与合规风险**：采集站模式直接涉及版权灰色地带，无法面向 NAS 厂商、商业硬件客户进行合规商业授权；
3. **发烧友需求脱节**：家庭影院及高价值用户普遍拥有 115、阿里网盘、夸克网盘或本地 NAS 原盘，急需一套纯净、4K 原画、精美海报墙的私有流媒体管理中心。

### 目标
1. **引入三态系统工作模式（System Mode）**：
   - `collect`（公共聚合模式）：保留现网采集能力与 TVBox 订阅，适合轻度开源用户；
   - `private`（纯私有媒体库模式）：彻底屏蔽采集中心，仅保留 WebDAV 存储挂载与 TMDB 刮削，**100% 纯净合规，零侵权风险**；
   - `hybrid`（混合模式）：私有网盘与网络采集源共存，本地有资源优先播 4K 原画，无资源时允许降级到采集源。
2. **基于 WebDAV 接入海量存储**：
   - 对接 Alist、本地 NAS、局域网共享目录，实现网盘（115/阿里/夸克/天翼）与本地硬盘的全盘挂载。
3. **TMDB 官方高清元数据自动刮削**：
   - 文件名正则解析（片名、年份、S01E01 季集）→ TMDB API 检索 → 自动填充原画海报、演职员表、背景剧照与评分。
4. **0 服务器带宽的 302 直链播放**：
   - 服务端仅做权限校验与直链转换，播放时直接 302 重定向至 WebDAV/网盘原画流，服务器零带宽开销。

---

## 2. 关键决策

| 决策点 | 选择 | 决策依据 |
| :--- | :--- | :--- |
| **模式切分策略** | 站点全局配置 `systemMode` | 解耦前后台视图：私有模式自动隐藏采集菜单与 API，消除违和感与合规风险 |
| **存储底座选型** | 标准 WebDAV 协议 | 不重写各个网盘复杂多变的逆向 SDK，依托 Alist 现成生态，以最小代码量支持所有主流网盘与本地存储 |
| **流媒体传输方案** | 302 直链重定向 (Redirect) | 服务端不经过视频流量（Pipe Stream 会吃死服务器带宽），客户端直连拉流，4K 秒开且零成本 |
| **刮削数据源** | TMDB 官方 API (The Movie Database) | 元数据质量最高、横竖版剧照全、演职员多语言支持完善，行业事实标准 |
| **TMDB 代理机制** | 支持自定义反代域名 / 内置 Proxy | 解决国内服务器访问 `api.themoviedb.org` 常见的 DNS 污染与握手超时 |
| **入库模型兼容** | 映射复用现有 `model.Movie` 表 | 沿用现有前台海报墙与播放器选集组件，最小化改造前端展示层 |

---

## 3. 架构设计与数据流

### 3.1 模式状态流转
```text
               ┌──────────────── SystemMode 配置 ───────────────┐
               │                                                 │
      ┌────────┴────────┐              ┌────────┴────────┐      ┌────────┴────────┐
      ▼                 ▼              ▼                 ▼      ▼                 ▼
   [collect 模式]                 [private 模式]                 [hybrid 模式]
   • 开放采集中心                  • 隐藏采集中心                  • 开放采集中心
   • 开放计划任务                  • 开放媒体存储(WebDAV)          • 开放媒体存储(WebDAV)
   • 仅消费采集源数据              • 开放刮削任务                  • 私有源优先播，采集源兜底
   • 输出 TVBox 订阅              • 关闭公开采集接口              • 支持多数据源切换
```

### 3.2 媒体扫描与刮削数据流
```text
WebDAV 挂载源 (Alist / NAS)
    │
    ▼ (PROPFIND 递归扫描)
视频文件发现 (.mp4 / .mkv / .ts)
    │
    ▼
文件名正则解析器 (提取：片名、年份、Season、Episode)
    │
    ▼
TMDB API 智能匹配 (/search/movie 或 /search/tv)
    │
    ├── 命中 ──► 获取高清海报、横版剧照、演职员、中文简介
    └── 未命中 ──► 降级为原始文件名作为基础片名展示
    │
    ▼
入库 movie / movie_detail_info 表 (PlayFrom 设为存储源名称，Link 设为内部串流路由)
    │
    ▼
用户点击播放 ──► 请求 /api/stream/play ──► 鉴权 ──► 302 重定向到 WebDAV 真实直链
```

---

## 4. 详细改造清单

### 4.1 数据库与模型层（Server）

1. **扩展全局配置 (`server/internal/model/manage.go` & `model/config.go`)**：
   - `SiteConfigRecord` 新增持久化字段：
     - `SystemMode`: 字符串枚举 `collect` | `private` | `hybrid`（默认 `collect`）；
     - `TmdbApiKey`: TMDB API Key（字符串）；
     - `TmdbProxyUrl`: TMDB API 代理地址（默认 `https://api.themoviedb.org`）。
2. **新增存储源模型 (`server/internal/model/storage_source.go`)**：
   - 字段：`id`, `name`, `type` (`webdav`), `endpoint`, `root_path`, `username`, `password`, `scan_status`, `last_scan_at`；
   - 在 `tables.go` 的 `AllModels` 注册 `&StorageSource{}` 确保 AutoMigrate 幂等升级。

### 4.2 刮削核心模块 (`server/internal/scraper/`)

1. **WebDAV 客户端 (`webdav.go`)**：
   - 使用标准 HTTP 客户端发起 PROPFIND 请求；
   - 递归过滤多媒体格式（`.mp4`, `.mkv`, `.ts`, `.mov`, `.iso`），跳过非视频文件与隐藏目录；
   - 支持根据文件 `ETag` 或 `LastModified` 执行增量扫描。
2. **文件名正则解析器 (`parser.go`)**：
   - 识别格式：`S01E02`, `s1e2`, `EP03`, `第04集`；
   - 提取年份：`(2023)`, `.2024.`；
   - 清洗常见压制标签：`2160p`, `4K`, `Remux`, `WEB-DL`, `HDR`, `H.265`, `x264` 等。
3. **TMDB 客户端 (`tmdb.go`)**：
   - 封装 `/search/movie`、`/search/tv`、`/movie/{id}`、`/tv/{id}`；
   - 内置请求重试与代理连接逻辑；
   - 图片路径拼接与 CDN 加速规则（支持转接自建图片反代）。
4. **入库流水线 (`pipeline.go`)**：
   - 组装电影（单集）或电视剧（多集归类聚合为同一部影片的不同分集）；
   - 批量落库至 `movie_detail_info` 与 `slave_movie_playlists`。

### 4.3 接口与路由层（Server）

1. **媒体存储管理接口 (`internal/handler/manage_storage.go`)**：
   - `GET /api/manage/storage/list`：查询存储源列表；
   - `POST /api/manage/storage/save`：新增/修改 WebDAV 配置（附带连通性测试）；
   - `DELETE /api/manage/storage/delete`：删除存储源及关联影片；
   - `POST /api/manage/storage/scan`：异步触发目录全量/增量刮削扫描。
2. **直链播放路由 (`internal/handler/stream.go`)**：
   - `GET /api/stream/play`：校验访问 Token（遵循前台登录开关策略），拼接 WebDAV 认证凭证或 Alist 签名直链，执行 `302 Found` 重定向。

### 4.4 前端界面层（Web）

1. **网站配置支持模式选择 (`web/src/app/manage/system/website/`)**：
   - 增加「系统运行模式」单选卡片：`公共聚合模式` / `私有媒体库模式` / `混合双模`；
   - 增加 TMDB 配置板块：`API Key` 输入框与 `代理反代地址` 输入框（附带测试连接按钮）。
2. **管理后台菜单动态自适应 (`web/src/app/manage/layout-view/index.tsx`)**：
   - 读取 `systemMode`：
     - 当处于 `private` 模式时，移除侧边栏「采集管理」分组，新增「媒体存储」分组（挂载管理、刮削任务）；
     - 当处于 `hybrid` 模式时，同时显示「采集管理」与「媒体存储」；
     - 当处于 `collect` 模式时，保持现网菜单不变。
3. **新增存储管理页面 (`web/src/app/manage/storage/`)**：
   - 存储源卡片列表：显示存储类型、URL、挂载状态、上次扫描时间与扫描进度；
   - 弹窗表单：支持填入 WebDAV 路径并提供【连接测试】按钮；
   - 扫描控制台：展示实时刮削日志、匹配成功率与未识别影片手动修正入口。

---

## 5. 验证与测试计划

1. **单元测试**：
   - 正则解析器测试：覆盖电影、季播美剧、国产连续剧、带杂乱后缀文件的解析命中率（测试文件 `parser_test.go`）；
   - TMDB 客户端测试：测试搜索、详情拉取及反代 URL 拼装。
2. **集成联调（关键路径走通）**：
   - **步骤 1**：本地启动 Alist 挂载测试目录，EcoHub 后台添加该 WebDAV 存储源，验证连通性测试接口；
   - **步骤 2**：放入示例影片（如 `奥本海默.Oppenheimer.2023.2160p.mkv` 和 `漫长的季节.S01E01.4K.mkv`），触发扫描；
   - **步骤 3**：观察控制台日志，验证 TMDB 刮削入库是否正确填充海报、演员、年份与分集；
   - **步骤 4**：在前台与后台影片管理中查看海报墙渲染效果；
   - **步骤 5**：点击播放，网络面板检查 `/api/stream/play` 是否返回 `302 Found`，播放器是否能正常接收原画视频流并流畅快进。
3. **模式切换测试**：
   - 在后台将模式在 `collect`、`private`、`hybrid` 之间来回切换，验证侧边栏菜单隐藏/展示无闪烁、无权限死循环。
