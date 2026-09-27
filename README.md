# 矩形波导模式核算服务

面向射频实验室的波导器件选型核对服务：把常用波导截面登记成**具名档案**，工程师点名档案 + 工作频率 + 模式指数即可一次性拿到**截止频率、传播状态、波导波长 / 衰减常数**；也支持不登记档案、临时提交尺寸现算。两条路径走同一套核算逻辑。

除逐点求值外，服务还能对一个截面把**整族 TE/TM 模式的截止谱整体枚举一遍**，直接回答选型时的第一个问题——「这个截面的单模工作区间是从哪到哪、升频后第二个冒出来的是谁（可能不止一个）」。具名档案与临时提交同样复用同一套枚举排序核算。

## 物理模型

矩形波导（填充介质非磁性，μr = 1）：

- 截止频率：`fc_mn = c / (2·√εr) · √((m/a)² + (n/b)²)`，c = 299792458 m/s
- 传播（f > fc）：波导波长 `λg = λ介质 / √(1 − (fc/f)²)`，其中 `λ介质 = c / (f·√εr)`
- 渐逝（f < fc）：衰减常数 `α = (2πf·√εr/c)·√((fc/f)² − 1)` Np/m，**不返回波导波长**
- 临界（f = fc，相对容差 1e-12）：状态 `critical`，轴向波数为零 —— 不判定为渐逝、不返回（无穷大的）波导波长，衰减常数恰为 0

输入约束（全部在计算前拦截，返回带原因的错误）：宽边 `a` 严格大于窄边 `b` 且都为正；`εr ≥ 1`；模式指数非负且 `(0,0)` 非法（物理上不存在该电磁波）。

内置样例档案 **WR-90**（a = 22.86 mm，b = 10.16 mm，空气填充）：主模 TE10 截止 ≈ 6.557 GHz，次模 TE20 截止 ≈ 13.114 GHz，整个 X 波段（8.2–12.4 GHz）落在单模传播区间。

### 模式谱与单模区间

- 横电模 **TE_mn**：m、n 非负且不同时为零（(0,0) 不存在）。
- 横磁模 **TM_mn**：要求 m ≥ 1 且 n ≥ 1；与同指数 TE_mn 共享截止频率，二者作为并列的简并条目同时留在谱里。
- 所有模式按 `fc_mn` 在频率轴上从低到高排成**非递减**谱；截止相同的模式（如 TE_mn/TM_mn，或 a = 2b 时的 TE20/TE01）作为相邻的并列条目保留，不会只留一个、丢掉另一个。
- **单模传播区间**为 `[主模截止, 次低截止)`：下界含（正好等于主模截止是临界起步），上界不含（一旦到达次低模式就已经至少两模同传）；低于下界无模可传。`dominant_modes` 是下界处的简并组，`next_modes` 是上界处的简并组——上界简并时该组含两个模式，其公共截止即区间上界。
- 模式指数理论上无限，枚举盒取 **0 ≤ m, n ≤ 20**（TE 除 (0,0)，TM 仅 m,n ≥ 1，共 840 条）。这个界不会漏掉决定单模区间上界的模式：对任何 a > b，全族截止波数最小值唯一为 1/a（TE10），而次小值只能是 `min(2/a, 1/b)`——TM 模的 m,n ≥ 1 ⇒ kc > 1/b；m ≥ 3 ⇒ kc ≥ 3/a > 2/a；n ≥ 2 ⇒ kc ≥ 2/b > 1/b。因此次低截止只可能由 TE20 或 TE01（或 a = 2b 时两者并列）实现，指数至多为 2；盒宽 20 只是让工程师能看到紧贴区间之后的几十个高阶模。该结论由 `TestBoundNeverMissesBandEdge` 对 1.01–1000 的宽窄比与 200×200 的暴力枚举逐点比对验证。

## 代码结构

```
cmd/server/main.go        入口：装配存储、内置档案、HTTP 服务（固定 :8080）
internal/physics/        核心公式：截止频率与传播状态、波导波长与衰减（纯函数，无状态）
internal/spectrum/       整族模式枚举、截止谱排序、主模/次低模与单模区间判定（独立模块）
internal/validate/       输入校验：尺寸、介电常数、模式指数、频率、档案名
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
| POST | `/profiles/:name/evaluate` | 档案逐点求值 `{mode_m, mode_n, frequencies_hz: [...]}` |
| POST | `/profiles/:name/spectrum` | 档案模式谱与单模区间（无请求体） |
| POST | `/evaluate` | 临时逐点求值：尺寸 + 介质 + 模式 + 频率一次提交 |
| POST | `/spectrum` | 临时模式谱：尺寸 + 介质一次提交 `{broad_dimension_m, narrow_dimension_m, relative_permittivity}` |

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

模式谱响应示例（`POST /api/v1/profiles/WR-90/spectrum`，谱只截取前几条）：

```json
{
  "profile": "WR-90",
  "single_mode_band": {
    "lower_frequency_hz": 6557140376.20,
    "upper_frequency_hz": 13114280752.41,
    "lower_inclusive": true,
    "upper_inclusive": false,
    "bandwidth_hz": 6557140376.20,
    "dominant_modes": [{"kind": "TE", "m": 1, "n": 0, "cutoff_frequency_hz": 6557140376.20}],
    "next_modes":     [{"kind": "TE", "m": 2, "n": 0, "cutoff_frequency_hz": 13114280752.41}]
  },
  "enumeration": {"max_mode_index": 20, "mode_count": 840},
  "spectrum": [
    {"kind": "TE", "m": 1, "n": 0, "cutoff_frequency_hz": 6557140376.20},
    {"kind": "TE", "m": 2, "n": 0, "cutoff_frequency_hz": 13114280752.41},
    {"kind": "TE", "m": 0, "n": 1, "cutoff_frequency_hz": 14753565846.46},
    {"kind": "TE", "m": 1, "n": 1, "cutoff_frequency_hz": 16145085787.91},
    {"kind": "TM", "m": 1, "n": 1, "cutoff_frequency_hz": 16145085787.91}
  ]
}
```

`a = 2b` 时 `next_modes` 同时含 TE20 与 TE01，二者 `cutoff_frequency_hz` 精确相等，即单模区间上界。截面合法性（a > b、εr ≥ 1 等）在枚举开始前由原有校验拦截，返回与既有接口一致的 400 错误。

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
- 批量部分失败隔离、同名冲突、删除未找到、并发请求互不串扰（`-race` 验证）
- **WR-90 单模区间**：下界精确等于 TE10 截止、上界精确等于 TE20 截止，整个 X 波段在 `[下界, 上界)` 内
- **宽边加倍 / εr 换成 4 ⇒ 区间上下界与区间宽度同时精确减半**
- 模式谱非递减；谱首截止即区间下界，其后第一个严格更高的截止即区间上界
- **a = 2b 简并**：TE20、TE01 同时出现在谱与 `next_modes` 中，公共截止即区间上界；TE/TM 同指数简并成对保留，(0,0) 与零指数 TM 不出现
- 枚举盒上界对 1.01–1000 宽窄比给出的上下界与 200×200 暴力枚举完全一致
- 模式谱的档案路径与临时路径逐位一致；非法截面在枚举前返回 400，未知名档案返回 404
