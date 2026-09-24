# New API（本仓库为二次开发版本）

> **本项目基于 New API 二次开发。**
> 原项目地址：[QuantumNous/new-api](https://github.com/QuantumNous/new-api)
> 原项目文档：[https://docs.newapi.ai/](https://docs.newapi.ai/)
>
> New API 是一个自托管的 AI 网关/聚合代理，将 40+ 上游模型服务（OpenAI、Claude、Gemini、Azure、AWS Bedrock 等）聚合到统一 API 之后，并提供用户管理、计费、限流与管理后台。项目的完整介绍、部署方式与使用文档请以上游原仓库为准。

本仓库仅在原项目基础上做了少量定制修改，其余功能、部署与使用方式均与上游一致。

## 本仓库的修改内容

### 1. 渠道贡献分成功能

新增渠道贡献分成能力，允许用户贡献渠道并按配置的比例参与收益分成，包含贡献开关、默认分成比例、单用户最大待处理数量等配置项，以及完整的结算流程。相关配置集成在管理后台的「运营设置」中。

### 2. 日志真实模型可见性开关（`LogUserRealModelEnabled`）

新增一个管理员开关，用于控制**普通用户**能否在自己的使用日志中看到：

- **真实路由模型**：模型映射后实际路由到上游的模型（`upstream_model_name` / `is_model_mapped`）；
- **上游返回模型**：上游实际返回的模型名（`response_model`）。

行为说明：

- 开关**开启**（默认）：保持原有行为，普通用户可在日志中看到上述字段。
- 开关**关闭**：上述字段会在普通用户视角的日志中被剥离；**管理员与 root 始终可见**，不受该开关影响。
- 开关位置：管理后台 →「运营设置」→「日志维护」区块。

实现要点：

- 后端在日志 `Other` 字段的用户可见性投影处（`model/log_other.go` 的 `formatLogOtherJSON`）统一剥离受控字段；
- 选项接线位于 `common/constants.go` 与 `model/option.go`（`Enabled` 后缀布尔开关）；
- 前端开关位于 `web/src/features/system-settings/maintenance/log-settings-section.tsx`，并已补齐各语言 i18n 文案；
- 默认值为 `true`，以保证升级后行为不变。

### 3. 注册码（`RegistrationCodeEnabled`）

新增注册码（准入码）能力，由管理员控制**前台用户注册时是否需要提供注册码**：

- 开关**关闭**（默认）：保持原有行为，任何人都可正常注册。
- 开关**开启**：用户在密码注册 / OAuth 首次建号时，必须持有有效的注册码才能创建账号；注册码本身不携带额度、分组等任何权益，只决定能否建号。

注册码由管理员在后台管理，支持：

- 批量生成或手动录入自定义码，可配置字符集（数字 / 大写 / 混合）、码长、前缀 / 后缀、排除易混字符；
- 每个码可设置启用 / 停用状态、最大使用次数（0 表示不限）、过期时间（0 表示不过期），并记录已用次数与最近使用时间。

实现要点：

- 后端模型与校验位于 `model/registration_code.go`，注册准入校验在 `controller/`（`registration_code.go` / `user.go` / `oauth.go` 等）落地，选项接线于 `common/constants.go` 与 `model/option.go`；
- 管理页面位于 `web/src/features/registration-codes/`（路由 `/registration-codes`），开关集成在管理后台 →「认证设置」的基础认证区块；
- 默认值为 `false`，以保证升级后行为不变（默认不强制注册码）。

### 4. 管理员统计看板（`/statistics`）

新增一个**仅管理员可见**的统计看板，聚焦「运营维度」的关键指标，与原有的额度/模型消耗看板互补：

- **今日 KPI 卡片**：
  - **日活人数**：当天去重调用过接口的用户数（每个用户当天多次调用只算一人）；
  - **今日注册 / 今日登录**：来自 `users.created_at` / `last_login_at`；
  - **今日访问人数**：按客户端 IP 每日去重，含未登录的匿名访客；
  - **今日调用次数**：当日消费日志（`type=2`）总次数。
- **近期趋势图**：近 N 天（7 / 14 / 30 可切换）的日活、访问、调用次数折线趋势。
- **IP 地区分布**：按访客 IP 解析出的地区聚合，展示各地区人数。

行为与隐私说明：

- 地区解析使用**离线 IP 库（ip2region）**，全程在本地完成，**不会外发用户 IP**；库缺失时优雅降级为「未知」，不影响主流程。
- 只落「地区聚合」与去重所需的 IP，不新增按用户的 IP 明细日志。

实现要点：

- 后端新增去重表 `stat_daily_active`（`model/statistics.go`），进程内内存缓存 + 唯一复合索引 `(day, kind, dedup_key)` 做跨实例去重；接口 `GET /api/statistics/overview` 由服务端 `middleware.AdminAuth()` 强校验（不依赖前端隐藏）；
- 调用埋点接入 `RecordConsumeLog`，访问埋点接入 `router/web-router.go` 的 `NoRoute` 链；离线地区解析封装在 `common/ipgeo/`；
- 前端页面位于 `web/src/features/statistics/`（路由 `/statistics`，管理员门禁），已补齐各语言 i18n 文案；
- 离线地区库 `data/ip2region.xdb` 已随仓库提交并内置进 Docker 镜像（`/opt/new-api/data/ip2region.xdb`，位于挂载卷之外），无需额外下载。

## 许可与归属

本仓库遵循原项目的开源许可协议，项目名称、品牌与作者归属（New API / QuantumNous）均保留自上游原项目。
