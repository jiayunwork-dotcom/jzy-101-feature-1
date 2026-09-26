# 矩形波导模式核算服务

面向射频实验室的波导器件选型核对服务：把常用波导截面登记成**具名档案**，工程师点名档案 + 工作频率 + 模式指数即可一次性拿到**截止频率、传播状态、波导波长 / 衰减常数**；也支持不登记档案、临时提交尺寸现算。两条路径走同一套核算逻辑。

## 物理模型

矩形波导（填充介质非磁性，μr = 1）：

- 截止频率：`fc_mn = c / (2·√εr) · √((m/a)² + (n/b)²)`，c = 299792458 m/s
- 传播（f > fc）：波导波长 `λg = λ介质 / √(1 − (fc/f)²)`，其中 `λ介质 = c / (f·√εr)`
- 渐逝（f < fc）：衰减常数 `α = (2πf·√εr/c)·√((fc/f)² − 1)` Np/m，**不返回波导波长**
- 临界（f = fc，相对容差 1e-12）：状态 `critical`，轴向波数为零 —— 不判定为渐逝、不返回（无穷大的）波导波长，衰减常数恰为 0

输入约束（全部在计算前拦截，返回带原因的错误）：宽边 `a` 严格大于窄边 `b` 且都为正；`εr ≥ 1`；模式指数非负且 `(0,0)` 非法（物理上不存在该电磁波）。

内置样例档案 **WR-90**（a = 22.86 mm，b = 10.16 mm，空气填充）：主模 TE10 截止 ≈ 6.557 GHz，次模 TE20 截止 ≈ 13.114 GHz，整个 X 波段（8.2–12.4 GHz）落在单模传播区间。

## 代码结构

```
cmd/server/main.go        入口：装配存储、内置档案、HTTP 服务（固定 :8080）
internal/physics/        核心公式：截止频率与传播状态、波导波长与衰减（纯函数，无状态）
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
| POST | `/profiles/:name/evaluate` | 档案求值 `{mode_m, mode_n, frequencies_hz: [...]}` |
| POST | `/evaluate` | 临时求值：尺寸 + 介质 + 模式 + 频率一次提交 |

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

重点用例（`internal/physics/physics_test.go`、`internal/api/server_test.go`）：

- **宽边加倍 ⇒ TE10 截止频率精确减半**（浮点严格相等）
- **εr 从 1 变 4 ⇒ 截止频率精确减半**（浮点严格相等）
- **f = fc 临界处理**：状态 `critical`、无波导波长字段、衰减为 0；两侧邻域分别为传导 / 渐逝
- 高阶模截止不低于主模；λg 随频率单调下降并逼近介质波长
- 批量部分失败隔离、同名冲突、删除未找到、并发请求互不串扰（`-race` 验证）
