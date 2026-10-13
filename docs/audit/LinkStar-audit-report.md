# LinkStar 全面审查报告

本文件是 2026-10-13 云主机审查的仓库副本，只记录结论，不含业务修复。每条问题的可粘贴正文在 [`issues/`](issues/README.md)。当时 `4evour/LinkStar` 是关闭了 Issues 的 fork，创建 Issue 的接口返回 410，因此这些正文先落在仓库里。

审查对象：仓库工作区（上游 [ZluxYao/LinkStar](https://github.com/ZluxYao/LinkStar)，当前检出版本号 `0.7.1`，见 `api/system_api/version.go` 的 `Version`）。

审查方式：在云主机上按文档做 CLI 构建和默认启动，跑 `go vet` 与约定的单元测试，再对认证、STUN/UPnP、DDNS、证书、反向代理、Webhook、更新做代码审查。能稳定打出来的问题都附了当次命令输出。没有改业务代码。临时复现测试已删除。

原始日志（含终端颜色码）：

- [`run-logs/build.stderr.txt`](run-logs/build.stderr.txt)
- [`run-logs/run1.stderr.txt`](run-logs/run1.stderr.txt)、[`run-logs/run1.stdout.txt`](run-logs/run1.stdout.txt)
- [`run-logs/run2.stderr.txt`](run-logs/run2.stderr.txt)
- [`run-logs/go-test.stdout.txt`](run-logs/go-test.stdout.txt)

---

## 1. 执行摘要

管理后台和导航主页能编过、能听在 `0.0.0.0:3333` 上，首页和管理页都返回 200。`go vet ./...` 通过。进程本身没有在启动时崩溃。

核心穿透能力在这台机器上没有起来，而且不是「没有公网、没有 UDP」。系统 DNS 能解析 STUN 主机，UDP Binding 请求能从 `stun.l.google.com:19302`、`stun.cloudflare.com:3478`、`stun.freeswitch.org:3478` 拿到响应。失败原因是默认 DNS 写死成 `114.114.114.114` 和 `119.29.29.29`，这两台从这里 UDP/53 超时；探测函数又把真实错误换成一句 `TCP connection failed to be established`。`InitSTUN` 失败后没有重试，打洞、UPnP 映射、NAT 类型检测都不会再启动。HTTP 服务继续跑，看起来像「已经启动」。

同一轮审查里，下面几件事能在本机稳定复现：

- 首次启动、密码还没设时，局域网任意主机可以调用 `POST /api/auth/setup` 把管理权抢走。
- `config/authConfig.json` 以 `0644` 落下 JWT 签名密钥。本机其他用户读到文件后可以伪造管理员 token，不必知道密码。已对活着的进程验证：伪造 token 访问受保护的 `/api/stun/config` 返回 200。
- 公网 IP 探测失败时函数仍返回成功，并把已经探到的公网 IP 覆盖成空字符串。
- 多个 UDP 转发共用一张只按「客户端地址」索引的会话表，后一个目标的包会被写进前一个内网连接。
- Webhook 代理地址非法时，`Send` 对空客户端解引用。调度器在独立 goroutine 里调用它且没有 `recover`，会把整个进程打崩。
- Web IP 来源不检查 HTTP 状态，也不检查正文是不是 IP。HTTP 500 的 HTML 会被当成地址返回。

总体判断：界面和模块拆分是认真做的（登录失败有次数限制、DDNS 凭证接口有脱敏、日志路径有白名单、自动更新对 GitHub 文件做 sha256、JWT 限定 HS256、证书私钥是 `0600`）。穿透和鉴权这两条主路径上有几处会在真实部署里直接失效或被旁路，应先修第 5 节列出的前六项。

---

## 2. 环境与启动过程

### 2.1 环境

| 项 | 值 |
| --- | --- |
| OS | Linux 6.12.94+ x86_64（云主机，主机名 `cursor`） |
| Go | go1.25.1 linux/amd64（与 `go.mod` 的 `go 1.25.1` 一致） |
| Node | v22.14.0（本次未改前端，未使用） |
| task | 未安装 |
| CGO | 启用（`CGO_ENABLED=1`），本次 `go build` 未显式关掉 |
| umask | `0022` |
| 出口网卡 | `enp0s3`，地址 `172.30.0.2`，网关 `172.30.0.1` |
| IPv6 | 无路由。访问 IPv6 IP 回显站全部 `network is unreachable` |
| UPnP | 无。日志：`未发现upnp网关` |
| 公网 STUN | 系统 DNS 可解析；UDP Binding 成功。见 2.4 |
| 内置 DNS | `114.114.114.114:53`、`119.29.29.29:53` UDP 查询 2 秒超时。`8.8.8.8:53` 正常 |
| 监听 | 程序固定 `0.0.0.0:3333`，本机可访问 |

构建没有缺证书、没有端口冲突。工作目录用的是 `/tmp/linkstar-audit/run` 和 `run2`，没有往仓库里写 `config/`、`data/`。

### 2.2 命令时间线

1. 阅读 `README.md`、`BUILD.md`、`Taskfile.yml`、`go.mod`、`main_cli.go`、`app.go`。CLI 入口是 `//go:build !desktop` 的 `main_cli.go`。前端 `web/*/dist` 已在仓库里，`go:embed` 直接打进二进制。没有 Dockerfile。`Taskfile.yml` 的发布任务调用 `powershell`，这台 Linux 上即使装了 task 也不能按文件原样跑。

2. 构建（退出码 0，产物约 34MB）：

```text
cd /workspace && go build -o /tmp/linkstar-audit/linkstar .
```

stderr 只有 `go: downloading ...`，没有编译错误。全文见 [`run-logs/build.stderr.txt`](run-logs/build.stderr.txt)。

3. 第一次默认启动（退出码 124，因为外层 `timeout 12` 发送了 SIGTERM，不是程序自己崩了）。工作目录 `/tmp/linkstar-audit/run`。

```text
cd /tmp/linkstar-audit/run && timeout 12 /tmp/linkstar-audit/linkstar
```

stdout 只有一行：

```text
[GIN] 2026/10/13 - 15:15:38 | 200 |      18.056µs |       127.0.0.1 | GET      "/linkstar/"
```

这是进程自己在 `isLinkStarBackendReady` 里探测管理页。stderr 全文见 [`run-logs/run1.stderr.txt`](run-logs/run1.stderr.txt)。去掉颜色码后的关键行：

```text
[info]  LinkStar Run
[info]  后端运行在：0.0.0.0:3333
[info]  管理后台运行在 http://127.0.0.1:3333/linkstar/
[info]  导航主页运行在 http://127.0.0.1:3333/
[info]  Auth / Home / Webhook / Cert / Proxy / Update / DDNS 模块初始化完成
[info]  [cert] 自签证书(id=1): 已生成自签证书：0 个域名 + 4 个本机地址，有效期至 2036-10-13
[info]  当前出口网卡：enp0s3(172.30.0.2 网关 172.30.0.1)
[warning] IP源探测失败 https://api6.ipify.org: dial tcp6 [...]:443: connect: network is unreachable
[warning] IP源探测失败 https://ipv6.icanhazip.com: ... network is unreachable
[warning] IP源探测失败 https://ipv6.ip.sb: ... network is unreachable
[warning] IP源探测失败 https://6.ident.me: ... network is unreachable
[info]  IP源探测成功 ip 34.196.179.219 https://ipv4.getip.cc
[info]  IP源探测成功 ip 18.233.210.77 https://ifconfig.me/ip
[info]  IP源探测成功 ip 52.2.175.83 https://myexternalip.com/raw
[info]  IP源探测成功 ip 54.236.146.182 https://ipv4.ip.sb
[info]  IP源探测成功 ip 34.228.201.179 https://4.ident.me
[info]  探测到公网出口: 242.14.30.107，终止扫描
[info]  当前NAT Router：[{1 10.0.6.171 private}]
[info]  TCP connection failed to be established     （连续 17 条，对应默认 STUN 列表）
[info]  当前使用stun服务器：
[warning] 未发现upnp网关
[warning] 没找到同网段的网关，使用第一个可用网关
[error] STUN 模块初始化失败：初始化 STUN 运行时失败: 没有可用的 STUN 服务器
[info]  收到退出信号：terminated ,正在保存配置文件
[info]  配置保存完成，程序退出
```

同一秒内五个 IPv4 回显站给出五个不同地址。路由扫描把 `242.14.30.107`（`240.0.0.0/4` 保留段）当成公网出口并停止。STUN 模块在报错后没有再打日志，直到进程被终止。

4. `go vet ./...` 退出码 0，无输出。

5. 单元测试（按仓库 `AGENTS.md`，不跑 `test/` 下会打外网的手工程序）：

```text
cd /workspace && go test ./api/... ./modules/... ./routers/... ./utils/...
```

退出码 1。失败的只有 `linkstar/modules/stun` 里三条会连公网的测试，其余包 `ok` 或 `[no test files]`。原文：

```text
--- FAIL: TestNatType (4.00s)
    nat_type_test.go:8: 启动时检测 NAT 类型失败: 网络基础数据未就绪: 从RFC 5780候选列表选择STUN服务器失败: 没有有效的 RFC 5780 候选服务器: stun.voipgate.com:3478: 解析失败: 解析 stun.voipgate.com 失败: lookup stun.voipgate.com: i/o timeout
        （stun.sonetel.com、stun.radiojar.com、stun.nextcloud.com、stun.sonetel.net、stun.sip.us、stun.dcalling.de、stun.freeswitch.org 同样 i/o timeout）
--- FAIL: TestRFC5780UDP (4.00s)
    rfc5780_test.go:10: selectRFC5780Server 失败: 没有有效的 RFC 5780 候选服务器: ... lookup ...: i/o timeout
--- FAIL: TestRFC5780TCP (4.00s)
    rfc5780_test.go:35: selectRFC5780Server 失败: ... lookup ...: i/o timeout
FAIL
FAIL	linkstar/modules/stun	12.494s
```

这三条测试没有假服务器，失败受当时 DNS 超时影响。同一次 `go test` 的 stdout 里还有一条预期内的错误日志（转发测试在断言「勾错 HTTPS」）：

```text
time="2026-10-13T07:15:44Z" level=error msg="连接内网目标失败 [127.0.0.1:35379]: 内网服务不是 HTTPS，却按 HTTPS 去连了——请取消该服务的「转发给内网时也用 HTTPS」勾选（原始错误：tls: first record does not look like a TLS handshake）"
```

6. 第二次启动在 `/tmp/linkstar-audit/run2`，用来打 HTTP。管理页、首页均 200。`GET /api/auth/status` 返回 `initialized:false`。随后用接口把密码设上并做了第 3 节里的鉴权复现。路由扫描再次停在保留地址：`探测到公网出口: 242.2.26.193`。进程在探针结束后被杀掉。stderr 见 [`run-logs/run2.stderr.txt`](run-logs/run2.stderr.txt)。

### 2.3 启动结论

卡在 STUN 初始化，不卡在编译、端口或证书。HTTP 平面（导航页、管理页、认证、自签证书、DDNS 模块加载）是通的。打洞平面没有服务器可选，`Runtime.Scheduler.StartAll` 根本不会被调用（它在 `InitSTUN` 成功返回之后，见 `modules/stun/init_stun.go` 第 112 行）。全仓库只有 `app.go` 的 `startModulesInBackground` 调用 `stun.InitSTUN`，失败就结束那个 goroutine，没有第二次。

UPnP 缺失、IPv6 不可达是这台云主机的网络限制，不是代码把网关探测写错。IPv6 回显失败被记成 warning，IPv4 回显仍继续，这个降级是合理的。

### 2.4 对照实验：网络其实打得通

默认 DNS（2 秒超时）：

```text
114.114.114.114:53   FAIL TimeoutError
119.29.29.29:53      FAIL TimeoutError
8.8.8.8:53           OK
```

系统解析器可以解析 `stun.freeswitch.org`。用系统解析器做的连通性：

```text
TCP stun.freeswitch.org:3478   OK
TCP stun.nextcloud.com:3478    OK
TCP stun.l.google.com:19302    超时（对端多半不收 TCP，不影响 UDP）
TCP stun.cloudflare.com:3478   超时
UDP stun.l.google.com:19302    OK，XOR-MAPPED 54.236.146.182
UDP stun.cloudflare.com:3478   OK，XOR-MAPPED 3.226.170.98
UDP stun.freeswitch.org:3478   OK，XOR-MAPPED 52.2.175.83
UDP stun.nextcloud.com:3478    OK，XOR-MAPPED 52.2.175.83
```

同一台机器、同一分钟里，STUN 映射地址和 HTTP 回显地址彼此也不相同。这是云出口多地址，不是某一行解析写错；代码侧的问题是「拿到什么就信什么」，见 LS-08。

把默认网络配置（内置 DNS）套进 `OutboundIface.DialTCP("stun.freeswitch.org:3478")` 的实测：

```text
DialTCP elapsed=4.003s err=解析 stun.freeswitch.org 失败: lookup stun.freeswitch.org: i/o timeout
getSTUNServerDelay elapsed=4.002s err=TCP connection failed to be established
```

4 秒等于两台内置 DNS 各 2 秒超时。17 台 STUN 服务器并行探测，所以日志在启动后大约 4 秒成批刷出 17 条同样的句子，真实原因被丢掉。

---

## 3. 问题清单

严重程度：致命（进程或核心数据立即不可用且难以恢复）、高、中、低、建议。

### LS-01 内置 DNS 不可达时，打洞模块永久起不来，日志还指向错误原因

- 严重程度：高
- 定位：
  - `modules/stun/outbound_iface.go` `DefaultDNSServers`（约第 307 行）、`dnsServers`（约第 311–320 行）、`resolve` / `lookupVia`（约第 324–393 行）
  - `modules/stun/stun_server.go` `getSTUNServerDelay`（约第 145–152 行）
  - `modules/stun/init_stun.go` `InitSTUN`（约第 46–79 行、第 112 行）
  - `app.go` `startModulesInBackground`（约第 128 行，唯一调用点）
- 根因：`dnsMode` 为空时不走系统 DNS，固定问 `114.114.114.114` 和 `119.29.29.29`。问失败就认为没有 STUN 服务器。`getSTUNServerDelay` 把 `DialTCP` 的错误换成不带原因、不带服务器名的 `TCP connection failed to be established`。`InitSTUN` 用 `errgroup` 收集这个错误后直接 return，调度器不会启动，也没有人再调用 `InitSTUN`。
- 复现：
  1. 确认 `114.114.114.114:53` 与 `119.29.29.29:53` 的 UDP 查询超时，同时系统 DNS 能解析 `stun.freeswitch.org`，且对该主机的 TCP 3478 或 UDP Binding 成功（本报告 2.4 的命令）。
  2. `go build -o /tmp/linkstar .`，在空目录执行 `./linkstar`。不要改 `config/stunConfig.json` 的 `network.dnsMode`（空字符串就是内置 DNS）。
  3. 约 4–8 秒后日志出现 17 条 `TCP connection failed to be established`，然后是  
     `STUN 模块初始化失败：初始化 STUN 运行时失败: 没有可用的 STUN 服务器`。
  4. `curl -s http://127.0.0.1:3333/api/version` 仍返回版本号。继续观察日志，不会再出现 STUN 服务器或打洞服务启动。
- 预期：至少有一台 UDP/TCP 可达的 STUN 服务器被选中；DNS 失败时日志写出超时的是哪台 DNS、哪台 STUN；网络恢复后自动再探。
- 实际：列表里每台服务器都被标成 TCP 连接失败；`bestStun` 为空；打洞调度器不启动；进程保持运行。
- 修复方向：`getSTUNServerDelay` 返回 `fmt.Errorf("连接 %s 失败: %w", srv, err)`。默认 DNS 在内置服务器都超时后退回系统解析器，或把「跟随系统」当成解析失败后的下一次尝试。服务器选择同时发 UDP Binding，不要只 `DialTCP`（不少服务器只听 UDP，本机对 `stun.l.google.com:19302` 就是 TCP 超时、UDP 成功）。`InitSTUN` 失败后按间隔重试，成功再 `StartAll`。列表里保留一两个覆盖面更好的 UDP 服务器作为兜底。

### LS-02 密码尚未设置时，任意能访问 3333 的主机可以抢走管理权

- 严重程度：高
- 定位：
  - `routers/enter.go` `Run`，`Addr: "0.0.0.0:3333"`（约第 104–112 行）
  - `routers/auth_routers.go` `AuthRouters`，`POST auth/setup` 挂在公开组（约第 17–19 行）
  - `modules/auth/store.go` `Setup`（约第 67–97 行）
- 根因：监听所有网卡，而 `initialized == false` 时设置密码的接口没有额外的本机限制、没有一次性引导令牌。谁先 POST 谁就成为唯一管理员。README 把这件事描述成「第一次进管理后台会引导设置密码」，没有说明同网段其他人可以抢先。
- 复现：
  1. 空目录启动 `./linkstar`。
  2. `curl -s http://127.0.0.1:3333/api/auth/status`  
     实际返回 `{"code":0,"data":{"authed":false,"initialized":false},"msg":"成功"}`。
  3. 在还没打开管理页的情况下，从另一台能访问该端口的机器执行  
     `curl -s -H 'Content-Type: application/json' -d '{"password":"<至少8位>"}' http://<主机>:3333/api/auth/setup`。
  4. 响应 `code:0` 且带 `data.token`。再次 `GET /api/auth/status`，`initialized` 变为 `true`。
  5. 真正的使用者再打开管理页，只能用别人设下的密码登录；删掉 `config/authConfig.json` 才能重来，而那时候密钥和密码哈希已经一起被换掉。
- 预期：设置密码只接受来自本机的连接，或要求用户在启动日志里抄一个一次性口令。
- 实际：本机复现时，未带任何凭证的 POST 直接返回可用 JWT，状态变为已初始化。
- 修复方向：`Setup` 只允许 `127.0.0.1` / `::1`，或启动时在日志打出一次性 setup token，请求里必须带上。绑定地址至少让「只听本机」成为一个显式选项（路线图里已有「管理端口可配置」，但那不能代替初始化窗口的来源限制）。

### LS-03 JWT 密钥以 0644 写入，本地用户可以伪造管理员 token

- 严重程度：高
- 定位：
  - `utils/utilsFile/utils_json.go` `WriteJsonFile`（约第 80–82 行，`os.WriteFile(..., 0644)`）
  - `modules/auth/auth_config.go` `createConfig`（约第 39–55 行）、`randomSecret`（约第 69–75 行）
  - `modules/auth/store.go` `ValidateToken`（约第 132–141 行）
  - `utils/jwt/enter.go` `ParseToken`（约第 32–40 行，只校验 HS256 签名和过期时间）
- 根因：第一次启动就生成 `jwtSecret` 并和密码哈希一起写入 `config/authConfig.json`。目录 `0755`、文件 `0644`（umask 022 下的默认）。`Setup` 不轮换这个密钥，只有改密码才轮换。校验只看签名。同机其他用户读文件即可签发 `sub=admin` 的 token。DNS 服务商 token 也在同样权限的 `config/ddnsConfig.json` 里。证书私钥本身是 `0600`，说明项目知道私钥该收紧，配置文件没有同样处理。
- 复现：
  1. 启动程序，等 `config/authConfig.json` 出现。`stat -c '%a' config/authConfig.json` 得到 `644`。
  2. 用文件里的 `jwtSecret` 做 HMAC-SHA256，header `{"alg":"HS256","typ":"JWT"}`，payload `{"sub":"admin","iat":现在,"exp":一小时后}`。
  3. `curl -H "Authorization: Bearer <伪造token>" http://127.0.0.1:3333/api/stun/config`。
  4. 本机实测 HTTP 200，正文是 STUN 配置 JSON。此时也可以先不设密码，把密钥抄走，等主人完成设置后再用同一密钥签发，直到有人修改密码。
- 预期：密钥和密码哈希只有运行用户可读；设置密码时轮换密钥，使事先抄走的密钥失效。
- 实际：伪造 token 通过了 `AuthMiddleware`。
- 修复方向：`config/` 用 `0700`，含密钥的 JSON 用 `0600`。已存在的 `0644` 文件在读取时收紧权限。`Setup` 成功时调用与 `ChangePassword` 相同的 `randomSecret()`。`randomSecret` 在 `crypto/rand` 失败时不要退回 `time.Now()` 的十六进制（`auth_config.go` 约第 72–74 行），直接让启动失败。

### LS-04 公网 IP 探测失败仍报成功，并清空已经得到的地址

- 严重程度：高
- 定位：
  - `modules/stun/network_local.go` `GetPublicIPInfo`（约第 17–34 行）、`GetPublicIP`（约第 48–84 行）
  - `modules/stun/network.go` `updateNetworkAddress`（约第 65–102 行）
- 根因：`GetPublicIP` 失败时 `GetPublicIPInfo` 用 `fmt.Printf` 打到 stdout（没有换行，也不进日志文件），然后 `return info, nil`。`updateNetworkAddress` 只把错误当失败；错误是 nil 就继续。空字符串不以 `10.` 开头，于是判定探测成功，把 `Runtime.Network.PublicIP` 写成空。若之前有地址，还会 `go updateNatRouter()`。`EmitPublicIPChanged` 会跳过空字符串，所以不会把空地址推进 DDNS，但运行中的公网地址已经丢了，门户和调度器读到的是空。另外 `GetPublicIP` 不使用已有的 `isNonPublicIP`，`updateNetworkAddress` 只拒绝前缀 `10.`，`192.168.0.0/16`、`172.16.0.0/12`、`100.64.0.0/10` 都会被当成公网写进 DDNS。
- 复现：
  1. 在 `stun` 包的测试里把 `network.dnsMode` 设为 `system`，先写入 `Runtime.Network.PublicIP = "203.0.113.10"`。
  2. 调用 `GetPublicIPInfo("127.0.0.1:1")`。本机因为出口绑定在 `172.30.0.2`，这条连接不是立即拒绝，而是拨号超时。
  3. 观察返回值：`err=<nil>`，`public=""`。stdout 出现  
     `获取真实公网ip失败连接STUN服务器失败: dial tcp4 172.30.0.2:0->127.0.0.1:1: i/o timeout`  
     （没有换行，和测试日志粘在一起）。
  4. 再调用 `updateNetworkAddress("127.0.0.1:1")`。实测 `ok=true`，`PublicIP` 从 `203.0.113.10` 变成空字符串。日志仍写  
     `通过 STUN 服务器 127.0.0.1:1 获取到的网络地址: LocalIP=172.30.0.2, PublicIP=`。
- 预期：连接失败返回 error；已有公网 IP 保持不变；非公网映射地址沿用 `isNonPublicIP` 拒绝。
- 实际：失败被当成一次成功的空更新。
- 修复方向：`GetPublicIPInfo` 把 `GetPublicIP` 的错误返回给调用方，日志改走 logrus。`updateNetworkAddress` 在 `PublicIP == ""` 或 `isNonPublicIP` 时返回 false，不要写字段。删掉只判断 `strings.HasPrefix(..., "10.")` 的分支。`Runtime.Network` 的读写加锁（见 LS-10）。

### LS-05 不同 UDP 转发共用一张会话表，包会进错内网目标

- 严重程度：高
- 定位：`modules/stun/forward.go` `udpSessions`（约第 117 行）、`ForwardUDP`（约第 119–154 行）。调用点 `modules/stun/stun.go` 约第 218 行，每个 UDP 洞都进这个包级 `sync.Map`。
- 根因：键只有 `remoteAddr.String()`，没有本地监听端口，也没有目标地址。同一个客户端源地址先打到服务 A、再打到服务 B 时，第二次 `Load` 命中，数据写进通往 A 的那条 `net.Conn`。
- 复现：
  1. 在 `127.0.0.1` 上开两个 UDP 接收端 A、B，再开一个本地 `UDPConn` 当作洞口。
  2. 用同一个伪造远端 `203.0.113.9:40000` 调用  
     `ForwardUDP(local, remote, []byte("to-A"), A)`，稍等，再  
     `ForwardUDP(local, remote, []byte("to-B"), B)`。
  3. 本机实测：A 收到 `to-A` 和 `to-B`，B 收到空。
- 预期：`to-B` 只出现在 B。
- 实际：`serverA got=["to-A" "to-B"]`，`serverB got=[]`。
- 修复方向：键改成 `localPort + remoteAddr`，或者按服务各持一张表。已有会话如果 `targetAddr` 变了，关掉旧连接再拨新的。`Write` 的错误要处理，避免死连接一直留在表里直到 30 秒读超时。

### LS-06 非法 Webhook 代理地址会让整个进程 panic

- 严重程度：高
- 定位：
  - `modules/webhook/sender.go` `Send`（约第 35–38 行）
  - `modules/stun/scheduler.go` `sendServiceWebhook`（约第 678–679 行）
- 根因：`client, err := buildClient(cfg.Proxy)` 之后没有判断 `err`。`url.Parse("://bad-proxy")` 失败时 `client` 为 nil，`sendOnce` 里 `client.Do` 空指针。调度器在 `go func()` 里调用 `webhook.Send`，没有 `recover`。Go 里未捕获的 panic 会终止进程。
- 复现：
  1. 调用 `webhook.Send(WebhookConfig{URL: "http://127.0.0.1:1/", Method: "GET", Proxy: "://bad-proxy"}, nil)`。
  2. 实测立刻  
     `panic: runtime error: invalid memory address or nil pointer dereference`，`recover` 能接住，说明 panic 发生在 `Send` 内部，而生产调用方没有接。
  3. 对应产品路径：在已登录的管理端把某条 STUN 服务的 Webhook 代理填成非法 URL 并保存；等该服务进入会发 Webhook 的状态。进程退出，不写「代理地址无效」。
- 预期：`Send` 返回 `代理地址无效`，服务继续跑。
- 实际：空指针 panic。
- 修复方向：`buildClient` 的错误直接返回。`http.ProxyURL` 再检查 scheme 是 `http` 或 `https`。调度器里的 goroutine 加 `recover` 只作为最后一道，不能代替返回错误。

### LS-07 路由层级把保留地址、回环和链路本地当成公网出口

- 严重程度：中
- 定位：`modules/stun/network_nat_router.go` `classifyIP`（约第 92–105 行）、`collectHops`（约第 70–86 行）。对照 `modules/stun/stun_server.go` `isNonPublicIP`（约第 192–202 行），两套分类不一致。
- 根因：`classifyIP` 只认识 RFC1918 和 `100.64.0.0/10`。其余全部是 `public`，包括 `240.0.0.0/4`、`169.254.0.0/16`、`127.0.0.0/8`、`224.0.0.0/4`、`0.0.0.0/8`。`collectHops` 一看到 `public` 就停止，而且不把这一跳记进结果。
- 复现：
  1. 启动程序，看路由扫描日志。第一次是  
     `探测到公网出口: 242.14.30.107，终止扫描`，结果只有 `[{1 10.0.6.171 private}]`。第二次是 `242.2.26.193`，同样只剩一跳私网。`242.0.0.0/8` 属于保留段，不是可路由公网。
  2. 对 `classifyIP` 的直接调用：

```text
242.14.30.107  classify=public   isNonPublic=false
169.254.1.1    classify=public   isNonPublic=true
127.0.0.1      classify=public   isNonPublic=true
224.0.0.1      classify=public   isNonPublic=true
0.0.0.1        classify=public   isNonPublic=false
192.168.1.1    classify=private  isNonPublic=true
100.64.0.1     classify=cgn      isNonPublic=true
```

- 预期：保留段、回环、链路本地、组播继续往下探，或标成未知并写进日志；界面上的 NAT 层数不要停在这种地址上。
- 实际：扫描在第一跳保留地址结束，NAT 层级被少算。
- 修复方向：让 `classifyIP` 和 `isNonPublicIP` 共用一个函数，补上 `IsLoopback`、`IsLinkLocalUnicast`、`IsMulticast`、`IsUnspecified` 和 `240.0.0.0/4`。这类地址不要触发「已经到公网」。

### LS-08 Web 来源的 IP 不校验，失败页和互相矛盾的回显都会被采纳

- 严重程度：中
- 定位：
  - `modules/ddns/ddns_syncrecord.go` `fetchIPFromWeb`（约第 397–421 行）、`resolveIP` 的 `IPSourceWeb` 分支（约第 154–170 行）
  - 同文件 `initDefaultIPSources`（约第 324–352 行）只用于排序，不要求各源结果一致
- 根因：`client.Get` 之后不看状态码，`TrimSpace` 后的正文原样返回。`resolveIP` 对自定义来源会 `net.ParseIP`，对 Web 来源不会。排序按延迟，谁先成功就用谁。
- 复现：
  1. `httptest` 返回 500 和 `<html>not-an-ip</html>`，调用 `fetchIPFromWeb(srv.URL, 4)`。实测 `err=<nil>`，返回值就是这串 HTML。
  2. 启动时的真实回显（同一次探测）：`34.196.179.219`、`18.233.210.77`、`52.2.175.83`、`54.236.146.182`、`34.228.201.179`。DDNS 会采用延迟最低的那一个，不会发现它们不是同一个地址。本机 STUN 映射同样分裂成至少三个地址，说明出口本身就不唯一；没有交叉验证时，写进 DNS 的记录是碰运气。
- 预期：非 2xx 或 `net.ParseIP` 失败则换下一个源；多个源结果不一致时不要更新记录，并在界面上说明。
- 实际：HTML 被当成 IP；多个不同的 IPv4 都被记成「探测成功」。
- 修复方向：限制正文长度后 `net.ParseIP`，并核对地址族。状态码必须是 200。内置列表至少两个源一致才采用。用户自定义 URL 拒绝链路本地和云元数据网段，避免已登录用户把这个客户端当成打内网的跳板（同机 `169.254.169.254` 一类地址）。

### LS-09 搜索历史对未登录访问者公开

- 严重程度：中
- 定位：
  - `routers/home_routers.go` `HomeRoutersPublic`（约第 16–17 行）
  - `api/home_api/search_history.go` `SearchHistoryGetView`（约第 16–21 行）
  - `api/home_api/get_home_config.go` `GetHomeConfigView`（约第 41 行，`SearchHistory` 塞进公开配置）
- 根因：主页配置按设计公开，但搜索历史跟着配置一起出去，另外还有一个专门的公开 GET。写接口在登录组里。导航页一旦通过打洞或反代暴露，历史关键词对互联网可读。
- 复现：
  1. 登录后 `POST /api/home/search-history/add`，正文 `{"keyword":"nas-admin-password-hint"}`。返回「已记录」。
  2. 去掉 `Authorization`，`GET /api/home/search-history`。实测  
     `{"code":0,"data":["nas-admin-password-hint"],"msg":"成功"}`。
  3. `GET /api/home/config` 的 `data.searchHistory` 同样是这个数组。
- 预期：历史只给已登录会话；公开主页配置不含 `searchHistory`。
- 实际：未登录可读。
- 修复方向：把 `home/search-history` 的 GET 挪进 `HomeRoutersProtected`。`GetHomeConfigView` 在未登录时把 `SearchHistory` 置空。主页若确实要在本机记住历史，放在浏览器本地，不要进公开 JSON。

### LS-10 公网 IP 和 NAT 链路没有锁，和 DDNS、调度器并发读写

- 严重程度：中
- 定位：
  - 写：`modules/stun/network.go` `updateNetworkAddress`（约第 90–91 行）、`updateNatRouter`（约第 112 行）、`refreshOutboundIface`（约第 136 行）
  - 读：`modules/ddns/ddns_syncrecord.go` `resolveIP`（约第 147 行，`stun.Runtime.Network.PublicIP`）、`modules/stun/scheduler.go`（约第 649、712 行）、`modules/stun/lookup.go`（约第 88 行）
- 根因：`STUNRuntime` 没有互斥锁。字符串和切片在一个 goroutine 里写、在别的 goroutine 里读，符合数据竞争。和 LS-04 叠在一起时，读到的可能是空字符串或撕裂的切片头。
- 复现：
  1. 打开两个 goroutine，一个循环调用 `updateNetworkAddress`，一个循环读 `stun.Runtime.Network.PublicIP` 并传给 `resolveIP`。
  2. `go test -race` 会报告对 `PublicIP` 和 `NatRouterList` 的并发读写。本次默认 `go test` 没加 `-race`，所以正式测试输出里没有这条；竞争从上述读写点可以直接看出来。
- 预期：读到的永远是某一次写完的完整快照。
- 实际：没有同步。
- 修复方向：网络状态用 `RWMutex`，对外只返回拷贝。不要在持锁时做 STUN 或 traceroute。

### LS-11 改密码没有失败次数限制

- 严重程度：低
- 定位：`modules/auth/store.go` `ChangePassword`（约第 150–170 行）对照 `Login`（约第 109–116 行）和 `modules/auth/login_limit.go`（窗口 3 分钟、最多 13 次）。
- 根因：登录失败会计数，修改密码的旧密码校验不计。调用方已经持有有效 token，所以这不是未登录爆破；偷到 token 的人可以无限制试旧密码，试中后 `ChangePassword` 会轮换密钥并把原主人踢下线。
- 复现：
  1. 登录拿到 token。
  2. 连续 `PUT /api/auth/password`，`oldPassword` 每次都错，`newPassword` 长度不少于 8。
  3. 第 14 次仍然返回「密码错误」，而不是登录接口那种「登录失败次数过多」。
- 预期：与登录共用同一限制器。
- 实际：`ChangePassword` 不调用 `globalLoginFailureLimiter`。
- 修复方向：旧密码不匹配时走 `loginAttemptFailed`。限制器触发时返回同一句「请稍后重试」。

### LS-12 反代和洞口转发默认跳过 TLS 校验

- 严重程度：低
- 定位：
  - `modules/proxy/handler.go` `newBackendTransport`（约第 150–151 行）`InsecureSkipVerify: true`
  - `modules/stun/forward.go` `ForwardTCP`（约第 76–82 行）同样跳过校验
  - `modules/proxy/probe.go` 探测后端时同样跳过
- 根因：注释写了内网多为自签。效果是同一二层上的中间人可以冒充 HTTPS 后端，管理流量和洞口流量都不会发现证书被换过。对家庭局域网这是明确取舍，但没有按站点关闭的开关。
- 复现：把站点后端指到一个证书 CN 不匹配的 HTTPS 服务，请求仍被转发，日志不报证书错误。单测 `forward_test.go` 里也用 `InsecureSkipVerify` 连洞口，说明这是当前契约。
- 预期：默认仍可兼容自签，但能改成「校验并信任上传的 CA」，或至少在界面上写明当前不校验。
- 实际：所有 HTTPS 后端都不校验。
- 修复方向：增加可选的后端 CA。未配置时保持现在的行为，避免升级后家中自签服务全部断开。不要默默改成严格校验。

### LS-13 文档和构建入口与实现不一致

- 严重程度：建议
- 定位：
  - `README.md` 开发者指南写技术栈含「lego（ACME）」。`go.mod` 没有 lego。实现是 `modules/cert/acme.go` 里的 `golang.org/x/crypto/acme`。
  - `README.md` 写 `task build` 构建当前平台桌面版。`Taskfile.yml` 的 `build` 依赖 `windows:build`，并且用 `powershell -Command New-Item` 创建目录。本机没有 task，也没有 powershell。
  - `README.md` 写「全新安装会自带一张自签证书」。这和启动日志一致，不是缺陷，只是有效期到 10 年后（本次到 2036-10-13），浏览器仍会提示不受信任。文档已经说明自签不被浏览器信任。
- 复现：在本机执行 `task build`，命令不存在。全文搜索 `lego` 只出现在 README，不出现在 Go import。
- 预期：README 的 ACME 库名和 Linux 构建命令与仓库一致。
- 实际：按 README 在 Linux 上不能用 task 完成发布构建；ACME 库名是错的。
- 修复方向：改 README。Linux 的 task 用 `mkdir -p`。桌面版继续说明需要 Wails，并且交叉编译不保证可用（`BUILD.md` 已经这么写了）。

### LS-14 三条 STUN 测试绑定公网 DNS，CI 和离线环境必失败

- 严重程度：建议
- 定位：`modules/stun/nat_type_test.go` `TestNatType`（第 5–8 行）、`modules/stun/rfc5780_test.go` `TestRFC5780UDP`（第 6–10 行）、`TestRFC5780TCP`（第 32–35 行）。
- 根因：测试直接 `selectRFC5780Server` / `DetectAndClassifyNat`，候选域名解析超时即 `t.Fatalf`。本次三条都在 4 秒时失败，错误是 `lookup ...: i/o timeout`。同包其他用假数据的 NAT 分类测试是通过的。
- 复现：`go test -count=1 -timeout 30s -run 'TestNatType|TestRFC5780' ./modules/stun/`。断网或 DNS 慢时失败；DNS 正常时又会真的对公网 STUN 发探测，结果随 NAT 变化，不能当回归断言。
- 预期：默认 `go test` 不访问公网。集成探测放到 `//go:build integration` 或环境变量后面。
- 实际：它们在普通测试列表里，本次把整个 `go test ./modules/...` 打成失败。
- 修复方向：候选服务器改成 `httptest` 做不到的部分就不要放进默认测试。需要真网卡的探测用 build tag 隔开。

---

## 4. 测试与覆盖缺口

已通过或没有测试的包：`api/stun_api`、`api/system_api`、`modules/auth`、`modules/cert`、`modules/ddns`、`modules/ddns/dns`、`modules/plainhttp`、`modules/proxy`、`modules/stun/test`、`modules/update`、`modules/webhook`、`routers`、`utils/pwd`。

没有 `*_test.go` 的包包括 `api/auth_api`、`api/cert_api`、`api/ddns_api`、`api/home_api`、`api/proxy_api`、`api/webhook_api`、`modules/home`、`routers` 里除门户以外的大部分路由、`utils/jwt`。

缺口和这次缺陷的对应关系：

| 缺口 | 后果 |
| --- | --- |
| 没有「内置 DNS 超时则 STUN 初始化失败并停止重试」的测试 | LS-01 能在海外或屏蔽 114 DNS 的网络里一直存在 |
| 没有「GetPublicIPInfo 失败必须返回 error」的测试 | LS-04 被 `fmt.Printf` 藏住 |
| `ForwardUDP` 没有两个目标的测试 | LS-05 被单连接测试掩盖 |
| `webhook.Send` 没有非法代理用例 | LS-06 是空指针而不是返回值 |
| `fetchIPFromWeb` 没有非 200、非 IP 用例 | LS-08 |
| 认证测试覆盖了登录次数，没有覆盖文件权限和 setup 的远程调用 | LS-02、LS-03 |
| `TestNatType` / `TestRFC5780*` 依赖公网 | 正常回归被网络波动染红，见 LS-14 |

`go vet ./...` 没有发现这些问题。它们都是行为错误，不是类型错误。

本次没有跑 `go test -race` 的全量（LS-10 需要长时间并发）。没有构建 `-tags desktop`：本机没有 Wails 桌面运行时，桌面入口也不是这次启动路径。没有改前端，所以没有重跑 `npm run build`。

---

## 5. 结论与优先修复顺序

进程能起来，页面能打开，模块初始化和退出保存是工作的。穿透在「内置 DNS 不可达」时会静默停用，而且日志把 DNS 超时说成 TCP 连接失败。鉴权在两个时间点是开的：密码还没设的时候谁都能设，密码设了之后同机用户仍能凭 `0644` 密钥伪造登录。转发和 DDNS 还有三处会把流量或地址送到错误的地方。

建议按这个顺序改，每一项都可以用上面的步骤做回归：

1. LS-01：DNS 失败要可见、要回退、要重试；STUN 探测增加 UDP。这是这次启动里唯一让核心功能没起来的原因。
2. LS-02 和 LS-03：初始化接口限制来源，配置文件改为 `0600`，设置密码时轮换 JWT 密钥。
3. LS-04：探测失败不要清空公网 IP，非公网地址不要写入。
4. LS-05：UDP 会话键带上本地端口和目标。
5. LS-06：Webhook 代理错误要返回，不能 panic。
6. LS-07、LS-08：路由分类和 IP 正文校验，避免把保留地址或 HTML 当成公网地址。
7. LS-09：搜索历史移出公开接口。
8. LS-10、LS-11、LS-14：补锁、补改密码限流、把公网测试移出默认 `go test`。

不需要为了这次审查改版本号。云主机没有 UPnP、没有 IPv6，这两条只作为环境限制记录，不记成代码缺陷。
