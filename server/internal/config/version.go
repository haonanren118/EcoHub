package config

// Version 产品版本号；可用 -ldflags "-X server/internal/config.Version=..." 在构建时覆盖。
// 与 web/package.json / git tag 对齐（如 2.7.1）。
var Version = "2.7.1"

// ProjectURL 开源仓库地址。三处消费方：
//   1) Telegram 欢迎 / 帮助文案中的项目地址跳转；
//   2) upgrade.githubRepoPath 由此推导「检查更新」所用的 GitHub 仓库
//      （https://github.com/<owner>/<repo> → api.github.com/repos/<owner>/<repo>）；
//   3) upgrade 在未显式配置镜像地址时的兜底镜像仓库前缀。
// 自托管二次开发场景下必须指向自己的 fork，否则「检查更新」会提示上游版本、
// 且「立即升级」会拉取上游镜像，覆盖本地定制改动。
const ProjectURL = "https://github.com/haonanren118/EcoHub"

// DefaultImageRepo 在线升级时拉取镜像的默认仓库（不含 tag）。
// 与 ProjectURL 同源：ghcr.io/<owner>/<repo>。可用环境变量 ECOHUB_IMAGE_REPO 覆盖。
const DefaultImageRepo = "ghcr.io/haonanren118/ecohub"
