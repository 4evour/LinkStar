# 审查 issue 正文

`4evour/LinkStar` 是 [ZluxYao/LinkStar](https://github.com/ZluxYao/LinkStar) 的 fork。审查落地时 GitHub Issues 是关闭的：

- `POST /repos/4evour/LinkStar/issues` 返回 **410**，`Issues has been disabled in this repository.`
- `PATCH /repos/4evour/LinkStar` 把 `has_issues` 设为 true 返回 **403**，`Resource not accessible by integration`。当前凭证改不了仓库设置。

仓库里已有的标签只有 GitHub 默认那一套（`bug`、`documentation`、`enhancement` 等），没有严重程度标签，所以没有新建标签。建议打开 Issues 之后按各文件末尾的「建议标签」打标。

总报告：[../LinkStar-audit-report.md](../LinkStar-audit-report.md)

| 文件 | 标题 | 建议标签 |
| --- | --- | --- |
| [LS-01.md](LS-01.md) | [LS-01][高] 内置 DNS 不可达时打洞模块永久起不来 | bug |
| [LS-02.md](LS-02.md) | [LS-02][高] 未设密码时任意主机可抢走管理权 | bug |
| [LS-03.md](LS-03.md) | [LS-03][高] JWT 密钥以 0644 落盘，可伪造管理员 token | bug |
| [LS-04.md](LS-04.md) | [LS-04][高] 公网 IP 探测失败仍报成功并清空已有地址 | bug |
| [LS-05.md](LS-05.md) | [LS-05][高] UDP 转发会话串扰，数据包进错内网目标 | bug |
| [LS-06.md](LS-06.md) | [LS-06][高] 非法 Webhook 代理地址导致进程 panic | bug |
| [LS-07.md](LS-07.md) | [LS-07][中] 路由扫描把保留地址当成公网出口 | bug |
| [LS-08.md](LS-08.md) | [LS-08][中] Web IP 来源不校验，失败页会被当成地址 | bug |
| [LS-09.md](LS-09.md) | [LS-09][中] 搜索历史对未登录访问者公开 | bug |
| [LS-10.md](LS-10.md) | [LS-10][中] 公网 IP 与 NAT 链路并发读写没有锁 | bug |
| [LS-11.md](LS-11.md) | [LS-11][低] 修改密码没有失败次数限制 | bug |
| [LS-12.md](LS-12.md) | [LS-12][低] 反代和洞口转发默认跳过 TLS 校验 | bug |
| [LS-13.md](LS-13.md) | [LS-13][建议] 文档与构建入口和实现不一致 | documentation |
| [LS-14.md](LS-14.md) | [LS-14][建议] 默认 STUN 测试绑定公网，离线必失败 | bug |

打开 Issues 之后，每个文件从「现象」起可直接贴进正文，第一行 `#` 标题去掉井号即可当作 issue 标题。
