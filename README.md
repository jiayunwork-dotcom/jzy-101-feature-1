# 矩形波导模式核算服务

面向射频实验室的波导器件选型核对服务：把常用波导截面登记成**具名档案**，工程师点名档案 + 工作频率 + 模式指数即可一次性拿到**截止频率、传播状态、波导波长 / 衰减常数**；也支持不登记档案、临时提交尺寸现算。两条路径走同一套核算逻辑。

除单点求值外，服务还能把一个截面的**整个模式谱**摊开：枚举全部 TE/TM 模式并按截止频率排序，直接回答**主模是谁、紧随其后的是谁（含简并并列）、单模工作区间从哪到哪**——无需逐频率试探。模式谱同样支持具名档案与临时截面两条路径，复用同一套枚举与排序核算。

## 物理模型

矩形波导（填充介质非磁性，μr = 1）：

- 截止频率：`fc_mn = c / (2·√εr) · √((m/a)² + (n/b)²)`，c = 299792458 m/s
- 传播（f > fc）：波导波长 `λg = λ介质 / √(1 − (fc/f)²)`，其中 `λ介质 = c / (f·√εr)`
- 渐逝（f < fc）：衰减常数 `α = (2πf·√εr/c)·√((fc/f)² − 1)` Np/m，**不返回波导波长**
- 临界（f = fc，相对容差 1e-12）：状态 `critical`，轴向波数为零 —— 不判定为渐逝、不返回（无穷大的）波导波长，衰减常数恰为 0

输入约束（全部在计算前拦截，返回带原因的错误）：宽边 `a` 严格大于窄边 `b` 且都为正；`εr ≥ 1`；模式指数非负且 `(0,0)` 非法（物理上不存在该电磁波）。

内置样例档案 **WR-90**（a = 22.86 mm，b = 10.16 mm，空气填充）：主模 TE10 截止 ≈ 6.557 GHz，次模 TE20 截止 ≈ 13.114 GHz，整个 X 波段（8.2–12.4 GHz）落在单模传播区间。

## 模式谱与单模区间

对给定截面，服务枚举全部 TE 模（指数不同时为零）与 TM 模（两个指数都 ≥ 1），各自用同一条截止公式算出截止频率，按**非递减**排成模式谱。谱中最低截止即**主模**（下界，含——正好等于主模截止是临界起步），第一个**严格更高**的截止即**次低模式**（上界，不含——到达时至少两个模式同时传导）。二者构成单模区间 `[lower, upper)`。若次低位置出现**简并**（如 a = 2b 时 TE20 与 TE01 截止相同，或任意 TE_mn / TM_mn 对），并列的模式会全部出现在 `next_modes` 里，上界取它们的公共截止，不随手丢弃任何一个。

**枚举上界为什么安全**：模式指数理论上无限大，枚举必须有界。记 `K = c/(2·√εr)`，则主模 TE10 截止为 `K/a`，TE20 截止恰为 `2·K/a`，故次低截止不超过 `2·K/a`；任何 `m ≥ 3` 的模式截止 ≥ `3·K/a`，任何 `n ≥ 2` 的模式截止 ≥ `2·K/b > 2·K/a`（因为 `b < a`），剩下的 `m ≤ 2, n ≤ 1` 候选（TE01、TE11/TM11、TE21/TM21）也都不低于 `min(fc20, fc01)`。**决定单模区间的模式指数永远满足 `m ≤ 2, n ≤ 1`，与宽窄比无关**，所以枚举上界 ≥ 2 即保证区间精确。默认上界取 8（远高于 2），让谱延伸到区间之外多个模式，便于判断「稍微超出会先撞上谁」；也可用 `max_index` 显式指定（接受 2–32，小于 2 会被拒绝而不是静默放宽，因为那会漏掉决定上界的模式）。

## 代码结构

```
cmd/server/main.go        入口：装配存储、内置档案、HTTP 服务（固定 :8080）
internal/physics/        核心公式：截止频率与传播状态、波导波长与衰减（纯函数，无状态）
internal/spectrum/       模式谱：TE/TM 模式枚举、按截止排序、主模/次低模式与单模区间提取
internal/validate/       输入校验：尺寸、介电常数、模式指数、频率、档案名、枚举上界
internal/profile/        具名档案的进程内并发安全存取（RWMutex）
internal/batch/          批量频率求值调度：有界并发、逐点隔离、保序
internal/api/            Gin 接口层：路由、DTO、错误映射
```

## API 一览（前缀 `/api/v1`）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查 |
| POST | `/profiles` | 登记档案 `{name, broad_dimension_m, narrow_dimension_m, relative_permittivity}`；同名返回 **409 PROFILE_CONFLICT**，不覆盖 |
| GET | `/profiles` | 列出全部档案（按名排序） |
| GET | `/profiles/:name` | 查看单个档案 |
| DELETE | `/profiles/:name` | 删除；不存在返回 **404 PROFILE_NOT_FOUND** |
| POST | `/profiles/:name/evaluate` | 档案求值 `{mode_m, mode_n, frequencies_hz: [...]}` |
| POST | `/evaluate` | 临时求值：尺寸 + 介质 + 模式 + 频率一次提交 |
| GET | `/profiles/:name/spectrum` | 档案模式谱与单模区间；可选 `?max_index=N`（2–32，默认 8） |
| POST | `/spectrum` | 临时模式谱：`{broad_dimension_m, narrow_dimension_m, relative_permittivity, max_index?}` |

求值响应示例（批量、逐点隔离，坏频率只影响自己那一格）：

```json
{
  "profile": "WR-90",
  "mode": {"m": 1, "n": 0},
  "cutoff_frequency_hz": 6557140376.202975,
  "results": [
    {"frequency_hz": 1e10, "state": "propagating", "guide_wavelength_m": 0.039707},
    {"frequency_hz": 5e9,  "state": "evanescent",  "attenuation_np_per_m": 88.91},
    {"frequency_hz": -3,   "error": {"code": "INVALID_FREQUENCY", "message": "..."}}
  ]
}
```

错误统一为 `{"error": {"code": "...", "message": "..."}}`，校验类 400、未找到 404、冲突 409。

模式谱响应示例（`GET /api/v1/profiles/WR-90/spectrum`，已截断）：

```json
{
  "profile": "WR-90",
  "broad_dimension_m": 0.02286, "narrow_dimension_m": 0.01016, "relative_permittivity": 1,
  "max_index": 8,
  "single_mode_range": {"lower_hz": 6557140376.202975, "upper_hz": 13114280752.40595,
                        "lower_inclusive": true, "upper_inclusive": false},
  "dominant_modes": [{"kind": "TE", "m": 1, "n": 0, "cutoff_hz": 6557140376.202975}],
  "next_modes":     [{"kind": "TE", "m": 2, "n": 0, "cutoff_hz": 13114280752.40595}],
  "spectrum": [
    {"kind": "TE", "m": 1, "n": 0, "cutoff_hz": 6557140376.202975},
    {"kind": "TE", "m": 2, "n": 0, "cutoff_hz": 13114280752.40595},
    {"kind": "TE", "m": 0, "n": 1, "cutoff_hz": 14753565846.456692},
    {"kind": "TE", "m": 1, "n": 1, "cutoff_hz": 16145085787.909725},
    {"kind": "TM", "m": 1, "n": 1, "cutoff_hz": 16145085787.909725}
  ]
}
```

## 运行

本地（Go 1.22）：

```bash
go build -o waveguide-server ./cmd/server && ./waveguide-server   # 监听 :8080
```

Docker（多阶段构建，运行态为 distroless 非 root 镜像）：

```bash
docker build -t waveguide-service .
docker run --rm -p 8080:8080 waveguide-service
```

## 测试

```bash
go test ./...                 # 本地
docker build --target test .  # 容器内跑测试，失败即构建失败
```

重点用例（`internal/physics/physics_test.go`、`internal/spectrum/spectrum_test.go`、`internal/api/server_test.go`、`internal/api/spectrum_test.go`）：

- **宽边加倍 ⇒ TE10 截止频率精确减半**（浮点严格相等）
- **εr 从 1 变 4 ⇒ 截止频率精确减半**（浮点严格相等）
- **f = fc 临界处理**：状态 `critical`、无波导波长字段、衰减为 0；两侧邻域分别为传导 / 渐逝
- 高阶模截止不低于主模；λg 随频率单调下降并逼近介质波长
- **WR-90 单模区间**：下界精确等于 TE10 截止、上界精确等于 TE20 截止，整个 X 波段落在区间内
- **宽边加倍 ⇒ 单模区间上下界与宽度同时精确减半**；**εr = 4 ⇒ 上下界精确减半**
- 模式谱非递减排列；谱首截止 = 区间下界；谱中第一个严格更高的截止 = 区间上界
- **简并处理**：a = 2b 时 TE20 与 TE01 都出现在谱与 `next_modes` 中，公共截止即区间上界；TE_mn/TM_mn 恒简并
- 枚举上界不变性：`max_index` 取 2…16 单模区间完全一致；档案与临时两条谱路径逐位一致
- 批量部分失败隔离、同名冲突、删除未找到、并发请求互不串扰（`-race` 验证）
