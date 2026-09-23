# fw/config

fw 专用配置库: 数据源仅 **YAML 配置文件** 与 **环境变量**, 替代 `github.com/linxlib/config`。

## 特性

- 默认读取 `config/config.yaml`(相对工作目录), 仅接受 `.yaml` 后缀, 显式传 `.yml`/`.json` 直接报错;
- 三种加载形态, target 可以是结构体、结构体的某个字段、甚至普通变量:
  - `LoadWithKey("server", &opt)` — 顶层 section;
  - `LoadWithKey("server.port", &port)` — 点号路径, 直接注入到任意变量;
  - `LoadByTags(&opt)` — 按字段上的 `inject:"<section>"` tag 批量注入(与 fw/inject 的 tag 语义一致, 支持 `-` 跳过);
- 环境变量优先于 yaml: 显式 `env:"XXX"` tag, 或按 `ENVPrefix`(缺省 `FW`) 派生 `FW_<SECTION>_<FIELD>`;
- 自动重载按 **section 内容 hash** 增量更新: 只有被修改的顶层 section 对应的内存配置会被重写,
  回调只收到变化的 key; 文件未变化时零解码开销; 单个目标重载失败时回滚其旧值。

## 用法

```go
import "github.com/linxlib/fw/config"

type ServerOpt struct {
    Host    string `yaml:"host" default:"0.0.0.0"`
    Port    int    `yaml:"port" default:"8080"`
    Timeout int    `yaml:"timeout" default:"30"`
}

func main() {
    c := config.New(&config.Option{
        AutoReload:         true,
        AutoReloadInterval: time.Second,
        AutoReloadCallback: func(key string, cfg any) {
            // 只有 key 对应的 section 被修改时才会收到回调
        },
    }) // 缺省 Files = ["config/config.yaml"], ENVPrefix = "FW"

    var opt ServerOpt
    _ = c.LoadWithKey("server", &opt) // section 注入到结构体

    var port int
    _ = c.LoadWithKey("server.port", &port) // 点号路径注入普通变量

    c.Close() // 停止轮询
}
```

环境变量覆盖(优先级从高到低):

1. 字段显式 `env:"XXX"` tag;
2. `<ENVPrefix>_<SECTION>_<FIELD>` 派生(如 `FW_SERVER_PORT`, ENVPrefix 为 `-` 时跳过);
3. `<SECTION>_<FIELD>` 无前缀派生。

内嵌结构体带 `anonymous:"true"` tag 时, 其字段的环境变量路径不包含结构体名。

## tag 说明

| tag | 说明 |
|---|---|
| `yaml:"xxx"` | yaml 字段名 |
| `default:"xxx"` | 零值时填充的默认值(yaml 字面量) |
| `required:"true"` | 缺失时报错 |
| `env:"XXX"` | 显式环境变量名 |
| `inject:"server"` | LoadByTags 时从 section `server` 注入; `-` 跳过 |
| `anonymous:"true"` | 内嵌结构体不进入 env 变量名路径 |

## 文件叠加

`Files` 按列表顺序叠加(后读的覆盖先读的)。若存在 `config/config.<env>.yaml`(环境取
`Option.Environment` → `CONFIG_ENV` → go test 为 `test` → 否则 `development`), 会叠加在
`config/config.yaml` 之后。

## 自动重载

开启 `AutoReload` 后, 由文件监听(平台相关)或每 `AutoReloadInterval` 的轮询触发检测:

1. 文件 modtime 未变 → 直接跳过;
2. 解析 yaml, 计算各顶层 section 的规范化 sha256, 与上次快照对比;
3. 只对 变化/新增/删除 的 section 重新解码, 写回其已注册目标(全量目标 `""` 随任一 section 变化刷新);
4. 单个目标失败回滚其旧值, 其余目标不受影响; 成功后更新快照并在锁外触发
   `AutoReloadCallback(变化的 key, target)`;
5. 检测到需要重载时在标准输出打印 `config: reload detected, changed sections: [...]`,
   列出本次发生变化的顶层 section(`Silent: true` 时不打印; 无变化时不打印)。

变更检测机制(平台相关):

- **Linux**: 用 inotify 监听配置目录, 文件被写/创建/移动时即时触发增量重载, 无需等待轮询周期;
  事件经 `watchCh` 合并突发(已有待处理信号时丢弃, reloadTick 幂等), inotify 不可用或无目录时静默回退轮询.
- **非 Linux(含 Windows / macOS)**: 无 inotify, 完全依赖每 `AutoReloadInterval` 的轮询兜底,
  行为与引入监听机制前一致.

也可手动触发: `changed, err := c.Reload()` 返回本次发生变化的 section 列表。

也可在 `New` 之后按需开启(幂等): `c.StartAutoReload(time.Second)`,
`AutoReloadCallback` 需在调用前设置; fw 引擎对应 `e.EnableConfigReload(interval)`。
