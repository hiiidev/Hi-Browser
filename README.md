# Hi Browser

> 面向多账号隔离、代理绑定和本地环境管理的桌面浏览器工具，支持 Windows、Linux 与 macOS。

[![Release](https://img.shields.io/github/v/release/hiiidev/Hi-Browser?sort=semver)](https://github.com/hiiidev/Hi-Browser/releases)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-blue)](https://github.com/hiiidev/Hi-Browser/releases)
[![Issues](https://img.shields.io/github/issues/hiiidev/Hi-Browser)](https://github.com/hiiidev/Hi-Browser/issues)

> [!IMPORTANT]
> 本项目在原项目基础上的新增功能与修复目前仅在 macOS arm64 环境完成实际运行测试。Windows 和 Linux 保留构建与适配代码，但相关新增能力尚未完成同等程度的实机验证，使用前请自行测试。

## 项目介绍

Hi Browser 用于在一台桌面设备上管理多个彼此隔离的浏览器实例。每个实例可以拥有独立的用户数据、指纹参数、代理出口、插件配置、标签和快捷启动码，适合多账号运营、跨境业务、本地测试与自动化场景。

项目的核心目标是：

- 一账号一实例，避免 Cookie、LocalStorage、IndexedDB 等数据互相污染。
- 一实例一代理，稳定维护账号与网络出口的对应关系。
- 统一管理浏览器内核、代理节点、插件、自动化脚本与运行状态。
- 配置和实例数据保存在本地，由使用者自行掌控和备份。

适用场景：

- 多账号环境隔离
- 跨境电商与社媒账号运营
- 独立代理出口测试
- 浏览器自动化与外部系统集成
- 团队统一维护内核和实例配置

## 原项目与本项目的关系

Hi Browser 基于开源桌面项目 [Ant Browser](https://github.com/black-ant/Ant-Browser) 继续开发，并推荐使用开源项目 [fingerprint-chromium](https://github.com/adryfish/fingerprint-chromium) 提供的浏览器内核：

- Ant Browser 提供原始桌面管理端基础，本项目保留原项目链接与致谢。
- 上游项目负责 Chromium 内核及指纹能力。
- Hi Browser 不修改原始 Chromium 内核，而是在其基础上提供桌面管理、实例隔离、代理连接、参数适配、自动化和跨平台发布能力。
- 内核版本与下载资产来自 [fingerprint-chromium Releases](https://github.com/adryfish/fingerprint-chromium/releases)。

感谢 Ant Browser 与 fingerprint-chromium 项目提供的开源基础。

## 在原项目基础上修复了什么

### macOS 兼容性

- 修复 Chromium 148 的 `CFBundleIconName` 优先读取 `Assets.car`，导致实例 Dock 角标不生效的问题。
- 为每个实例生成稳定 Bundle ID 和独立派生 App，Dock 可以显示不同的编号与颜色角标。
- 使用版本化缓存、`plutil`、扩展属性清理和重新签名，原始 Chromium App 保持不变；派生失败时自动回退原内核并显示警告。
- 修复 `-AppleLanguages`、`-AppleLocale` 被 Chromium 当作网址打开的问题。
- 按实例 Bundle ID 写入独立语言偏好，使网页请求、`navigator.language`、Intl locale 与界面语言保持一致。
- 所有 macOS 实例统一使用隔离免提示存储，避免访问系统密码存储时出现授权弹窗。
- 修复 macOS 导入本地浏览器内核时的路径识别与崩溃问题。
- 修复安装版把配置写入只读 `.app` 的问题，运行数据统一写入用户状态目录。

### 指纹参数兼容性

- 适配 fingerprint-chromium 144+ 与 148 的参数变化，清理已经失效的旧 GPU/WebGL 参数。
- GPU 指纹默认由 seed 驱动的真实参数集生成，也可以显式切换为真实 GPU。
- 保存和启动前按当前内核能力规范化参数，并向用户展示兼容性调整结果。
- 保留 `--lang`、`--accept-lang` 和 `--timezone`，不通过 JavaScript 注入伪造语言或时区。
- 时区列表按当前日期计算 UTC 偏移，正确处理纽约、伦敦、悉尼等地区的夏令时。

### 启动与代理稳定性

- 增强浏览器启动就绪检测、调试端口恢复、会话恢复和延迟打开启动页逻辑。
- 修复代理连接栈自动混用导致的测速结果与实际启动链路不一致问题。
- 实例启动、测速、真实连通性、IP 健康、预热和代理下载统一遵守当前连接栈。
- 代理异常时支持仅本次直连启动，不修改实例原有代理配置。
- 完善 Xray、sing-box、Mihomo 和自动化 Node 运行时的跨平台发现、校验与打包。

### 内核安装安全

- 内核下载不再依赖客户端直接抓取 GitHub Releases 页面，改用静态 Manifest、ETag 缓存和内置回退数据。
- 下载先写入 `.part`，解压到 staging 目录，校验通过后再原子安装。
- 解压过程拒绝路径穿越、符号链接和硬链接，并限制文件数量与总尺寸。
- 支持 SHA-256 校验；上游未提供独立校验值时会明确标记为“仅本地完整性校验”。
- 下载失败、取消或验证失败不会注册残缺内核。

## 在原项目基础上新增了什么

以下新增能力当前只经过 macOS arm64 环境测试；Windows 和 Linux 的实际表现仍需进一步验证。

| 模块 | 新增能力 |
| --- | --- |
| 实例管理 | 创建、编辑、启动、停止、重启、复制、回收站、批量操作和独立系统图标角标 |
| 环境隔离 | 每实例独立用户数据、指纹参数、代理、插件、标签、关键字和分组 |
| 代理池 | 节点导入、分组、绑定、预热、测速、真实连通性和 IP 健康检测 |
| 多连接栈 | Xray + sing-box 组合栈，以及独立 Mihomo 栈 |
| 内核管理 | 自动发现、下载、校验、安装、切换默认版本、保留旧版本和回滚 |
| 指纹配置 | 跨平台预设、seed、语言、时区、屏幕、硬件、Canvas、Audio、字体和网络策略 |
| 快捷启动 | 实例 Code、`Ctrl + K` 搜索以及外部 Launch API |
| 自动化 | Playwright/CDP 脚本导入、脚本包、目标实例选择、运行记录和外部调用 |
| 插件管理 | 插件安装、导入、启停、删除、实例限制和单实例配置 |
| 实例迁移 | 配置与完整用户数据导出为 ZIP，并导入为新实例 |
| 跨平台发布 | Windows 安装包/便携包、Linux deb/tar.gz、macOS unsigned DMG |

## 界面预览

### 控制台

<img src="images/readme/001-首页.png" alt="Hi Browser 控制台" width="100%" />

### 实例列表

<img src="images/readme/002-实例列表.png" alt="浏览器实例列表" width="100%" />

### 代理池配置

<img src="images/readme/003-设置代理池.png" alt="代理池配置" width="100%" />

### 代理生效验证

<img src="images/readme/004-自定义代理.png" alt="代理生效验证" width="100%" />

## 快速开始

### 支持平台

- Windows 10 / 11，64 位
- Linux amd64 / arm64
- macOS amd64 / arm64，当前提供 unsigned 测试包

建议至少准备 8 GB 内存和 2 GB 可用磁盘空间。

### 下载运行

1. 前往 [Releases](https://github.com/hiiidev/Hi-Browser/releases) 下载对应平台版本。
2. Windows 可选择 NSIS 安装包或便携 ZIP。
3. Linux 可安装 deb，或解压 tar.gz 后运行。
4. macOS 打开 DMG，将 `Hi Browser.app` 拖入 `Applications`；如果 Gatekeeper 拦截，请先核对来源，再通过 Finder 右键“打开”或“系统设置 → 隐私与安全性”主动放行。
5. 首次进入“内核管理”，确认版本、平台、架构和下载来源后安装浏览器内核。
6. 在“代理池配置”添加节点，再创建实例并绑定代理。

Hi Browser 不会自动删除 `com.apple.quarantine`，也不会绕过 Gatekeeper。

### 第一次使用建议

1. 安装或导入 fingerprint-chromium 内核。
2. 导入代理节点并完成测速与 IP 健康检查。
3. 新建浏览器实例，设置指纹、代理、标签和启动参数。
4. 启动实例并访问 IP 检测网站，确认实际出口。
5. 后续使用固定实例与固定代理，避免账号环境频繁变化。

## 代理连接栈

`browser.default_connector_type` 只允许两种模式：

- `xray`：Xray + sing-box 组合栈。Xray 负责 vmess、vless、trojan、shadowsocks 和链式代理；sing-box 负责 hysteria2、tuic、anytls 等协议。
- `mihomo`：独立 Mihomo 栈，需要桥接的代理统一由 Mihomo 处理。

两套连接栈不能自动混用。实例启动、测速、真实连通性、IP 健康、预热和代理下载必须使用当前选择的连接栈。

## 数据目录

macOS 安装版的运行数据位于：

```text
~/Library/Application Support/hi-browser/
├── config.yaml
├── chrome/              # 已安装浏览器内核
├── download-cache/
└── data/
    ├── app.db           # 实例、代理、内核等配置
    ├── automation/
    ├── cache/
    └── <profile-id>/    # 每实例浏览器用户数据
```

Linux 安装版默认使用 `$XDG_DATA_HOME/hi-browser`；未设置 `XDG_DATA_HOME` 时使用 `~/.local/share/hi-browser`。升级时，如果新目录不存在而同级旧 `ant-browser` 数据目录存在，应用会自动将旧目录迁移为 `hi-browser`；如果两个目录同时存在，则保留两者且不会覆盖新目录。

源码开发模式的相对配置默认位于仓库目录。不要提交 `data/app.db`、实例目录或其他真实用户数据。

## 从源码运行

### 环境要求

- Go
- Node.js 与 npm
- Wails CLI 2.13+
- 对应平台的原生构建工具

### macOS / Linux

```bash
make deps
make dev          # Wails + Vite 热更新
make dev-stable   # 使用已构建的前端资源
make test
make check
make build
```

### Windows

```powershell
bat\dev.bat
bat\dev.bat stable
bat\publish.bat -Target WINDOWS -WindowsFormat INSTALLER
bat\publish.bat -Target WINDOWS -WindowsFormat PORTABLE
bat\publish.bat -Target WINDOWS -WindowsFormat BOTH
```

### GitHub 自动打包与发布

推送 `v*` 标签后，GitHub Actions 会并行构建 macOS、Windows 和 Linux，并将产物上传到同名 GitHub Release：

| 平台 | GitHub 产物 |
| --- | --- |
| macOS | `HiBrowser-arm64.dmg`、`HiBrowser-amd64.dmg` |
| Windows | NSIS 安装包、amd64 便携 ZIP |
| Linux | `hi-browser_<version>_amd64.deb`、`hi-browser_<version>_arm64.deb` 与对应 tar.gz |

发布前先将 `wails.json` 中的 `info.productVersion` 更新为目标版本并提交，然后推送对应标签：

```bash
git add wails.json
git commit -m "chore: release v1.4.2"
git push origin master

git tag v1.4.2
git push origin v1.4.2
```

也可以在仓库的 `Actions` 页面手动运行对应平台的 Publish 工作流，并通过 `version` 输入临时覆盖打包版本。标签发布需要仓库在 `Settings → Actions → General → Workflow permissions` 中允许 `Read and write permissions`，以便工作流创建或更新 Release。

代理运行时按平台放在 `bin/<os>-<arch>/`。固定来源记录在 `publish/runtime-sources.json`，哈希清单记录在 `publish/runtime-manifest.json`。

## 自动化脚本包

可提交的示例脚本位于：

```text
backend/internal/automation/demo-library/
```

用户脚本运行数据位于：

```text
data/automation/scripts/<script-id>/
```

对外脚本包采用以下结构：

```text
<script-id>/
├── automation.script.json
├── index.cjs
└── 其他辅助文件
```

`automation.script.json` 保存元数据和默认参数，`entryFile` 可以指向目录内的其他相对路径。

## 常见问题

### 应用启动后没有浏览器窗口

先确认已经安装可用于当前平台和架构的浏览器内核，再检查实例运行警告、调试端口和用户数据目录权限。

### 代理测速成功，但实例出口不一致

检查 `default_connector_type`。Hi Browser 不会在 `xray` 组合栈和 `mihomo` 栈之间自动混用；切换连接栈后需要重新测速。

### 多个账号如何避免串号

建议一账号一实例、一实例一稳定代理，不要复用用户数据目录，也不要频繁修改同一实例的出口地区、语言和时区。

### macOS 为什么显示 unsigned 或被 Gatekeeper 拦截

当前 macOS 包用于内部测试，没有 Apple Developer ID 公证。请先核对 Release 来源和 SHA-256，再通过系统提供的主动放行入口打开。

### macOS 提示“应用已损坏，无法打开”

只有在确认 DMG 来自本仓库 Release、且文件校验值可信后，才可以移除 macOS 下载隔离属性。先将应用拖入“应用程序”，关闭应用，然后执行：

```bash
sudo xattr -rd com.apple.quarantine "/Applications/Hi Browser.app"
```

重新打开应用即可。该命令会移除系统下载隔离标记，相当于绕过本次 Gatekeeper 隔离检查；不要对来源不明的应用执行。

## 文档

- [完整更新日志](CHANGELOG.md)
- [macOS 发布说明](publish/mac/README.md)
- [Linux 发布说明](publish/linux/README.md)
- [浏览器内核 Manifest](browser-core-manifest.json)

## 贡献与反馈

欢迎通过 Issue 和 Pull Request 参与改进：

- Bug 反馈请附带应用版本、操作系统、复现步骤和必要日志。
- 功能建议请说明业务场景、预期行为和当前限制。
- 较大改动建议先创建 Issue 对齐需求。

相关入口：

- [Releases](https://github.com/hiiidev/Hi-Browser/releases)
- [Issues](https://github.com/hiiidev/Hi-Browser/issues)
- 社区支持：<https://linux.do/>

## License

当前仓库暂未附带独立 `LICENSE` 文件。使用上游 fingerprint-chromium 内核时，请同时遵守其项目许可证和 Chromium 相关许可证。
