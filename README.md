# CamLink

CamLink 是一个面向安防/摄像头场景的 RTSP 转 Web 实时流服务，支持通过 **MSE / HLS / WebRTC** 在浏览器中播放视频，并在原项目基础上扩展了分享、鉴权、录像、中文化与车辆检测能力。

## 项目来源与声明

本项目基于开源项目 **RTSPtoWeb** 进行功能性扩展：
- 原项目地址：https://github.com/deepch/RTSPtoWeb
- 原项目 License：MIT

本仓库保留并遵循原项目的 MIT 许可证，详见 `LICENSE.md`。

## 安全说明

仓库只保留脱敏样例配置 `config.example.json`。
真实摄像头地址、账号密码、分享密钥等敏感信息必须保存在本地未跟踪文件 `config.local.json` 中。

初始化本地配置：

```bash
cp config.example.json config.local.json
```

然后编辑 `config.local.json`，填入你的 RTSP 地址与本地管理员配置。`config.local.json` 已被 `.gitignore` 忽略，不会进入仓库。

## 主要功能

- RTSP/RTMP 输入，Web 端多协议播放（MSE/HLS/WebRTC）
- 全局设置中心（录像参数、语言、分享策略、管理员安全项）
- 即时录像（REC/STOP）
- 分享视频（时效、密码、连接数、二维码）
- 管理后台登录鉴权
- 界面中文/英文切换
- 车辆驶入检测开发框架
  - Phase 1: 配置页、服务联通、Docker 骨架
  - Phase 2: SQLite 事件库、时间段查询、CSV 导出、测试事件写入

## 快速开始（Docker / OrbStack）

### 1. 准备本地配置

```bash
cp config.example.json config.local.json
```

### 2. 构建并启动

```bash
make compose-up
```

默认端口：
- Web: `http://127.0.0.1:8083`
- RTSP: `:5541`
- Detector: `:8091`

### 3. 查看日志

```bash
make compose-logs
```

### 4. 停止服务

```bash
make compose-down
```

## 常用开发命令

- `make docker-build`: 构建单容器镜像 `camlink:dev`
- `make docker-run`: 以标准端口启动单容器，挂载 `config.local.json`
- `make compose-up`: 启动主服务 + detector 两服务
- `make compose-up-local`: 用备用端口启动，避免与现有容器冲突
- `make docker-test`: 构建、启动、执行 `test.curl` / `test_multi.curl` 后自动停止

## 管理后台

- 登录页：`http://127.0.0.1:8083/login`
- 首次建议进入“全局设置”立即修改管理员密码
- 如果你已经公开过旧配置，请立即更换摄像头密码，并视情况重新设置分享密钥

## 目录说明

- `*.go`: 后端核心、流处理、HTTP API
- `web/templates`: 页面模板
- `web/static/js`: 前端逻辑与 i18n
- `services/detector`: 独立检测服务骨架
- `docs/vehicle-entry-detection-plan.zh-CN.md`: 车辆检测开发方案
- `config.example.json`: 脱敏样例配置
- `config.local.json`: 本地真实配置（不入库）

## License

本项目遵循 MIT License，保留原项目许可与版权信息。
