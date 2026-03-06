# CamLink

CamLink 是一个面向安防/摄像头场景的 RTSP 转 Web 实时流服务，支持在浏览器中通过 **MSE / HLS / WebRTC** 播放视频，并在原项目基础上扩展了分享、鉴权、录像与中文化能力。

## 项目来源与声明

本项目基于开源项目 **RTSPtoWeb** 进行功能性扩展：
- 原项目地址：https://github.com/deepch/RTSPtoWeb
- 原项目 License：MIT

本仓库保留并遵循原项目的 MIT 许可证（见 `LICENSE.md`）。

## 主要功能

- RTSP/RTMP 输入，Web 端多协议播放（MSE/HLS/WebRTC）
- 全局设置中心（WebUI）
  - 录像保存路径、格式、命名规则、最大时长
  - 默认界面语言（中文/英文）
  - 分享默认有效期与连接上限
  - 管理员账号与密码管理
- 即时录像
  - 单路播放页新增 `REC/STOP` 按钮
  - 录制文件按规则自动保存
- 分享视频（独立播放器链接）
  - 设置有效时间（分钟）
  - 自动生成 4 位分享密码
  - 控制同时访问连接数（1-5）
  - 生成分享链接与二维码（移动端可直接查看）
- 安全增强
  - 管理后台登录鉴权（会话 Cookie）
  - 分享访问令牌签名校验，防止绕过分享直接拉流
  - 流媒体接口统一鉴权链路
- UI 中文化
  - 默认中文
  - 中文 / English 快速切换

## 快速开始（推荐：Docker / OrbStack）

> 适合本地没有 Go/gofmt 环境的开发者。

### 1) 构建镜像

```bash
make docker-build
```

### 2) 启动服务

```bash
make docker-run
```

默认映射：
- HTTP: `http://127.0.0.1:8083`
- RTSP: `:5541`

默认配置文件挂载：`./config.json -> /config/config.json`  
默认录像目录挂载：`./save -> /app/save`

### 3) 查看日志

```bash
make docker-logs
```

### 4) 停止服务

```bash
make docker-stop
```

## 一键冒烟测试（无 Go 环境）

```bash
make docker-test
```

该流程会自动：构建镜像 -> 启动容器 -> 执行 `test.curl` 与 `test_multi.curl` -> 停止容器。

## 管理后台与默认账号

首次可用 `config.json` 中的账号访问后台。建议启动后立即在“全局设置 -> 安全设置”中修改管理员密码。

登录页：
- `http://127.0.0.1:8083/login`

## 分享功能使用

1. 在 Dashboard 或 Streams List 卡片点击“分享视频”  
2. 设置：有效时间、连接上限、频道  
3. 系统返回：分享链接 + 4 位密码 + 二维码  
4. 访客打开分享链接进入独立播放器页，不进入管理后台

## 目录说明

- `*.go`：服务端核心与 API
- `web/templates`：页面模板
- `web/static/js`：前端逻辑（含 i18n）
- `web/static/css`：样式
- `config.json`：运行配置
- `Makefile`：本地与 Docker 任务入口

## License

本项目遵循 MIT License，保留原项目许可与版权信息，详见 `LICENSE.md`。
