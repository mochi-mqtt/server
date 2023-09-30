# Mochi-MQTT Server

<p align="center">
    
![build status](https://github.com/mochi-mqtt/server/actions/workflows/build.yml/badge.svg) 
[![Coverage Status](https://coveralls.io/repos/github/mochi-mqtt/server/badge.svg?branch=master&v2)](https://coveralls.io/github/mochi-mqtt/server?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/mochi-mqtt/server)](https://goreportcard.com/report/github.com/mochi-mqtt/server/v2)
[![Go Reference](https://pkg.go.dev/badge/github.com/mochi-mqtt/server.svg)](https://pkg.go.dev/github.com/mochi-mqtt/server/v2)
[![contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat)](https://github.com/mochi-mqtt/server/issues)

</p>

[English](README.md) | [简体中文](README-CN.md)

🎆 **mochi-co/mqtt 现在已经是新的 mochi-mqtt 组织的一部分。** 详细信息请[阅读公告.](https://github.com/orgs/mochi-mqtt/discussions/271)


### Mochi-MQTT 是一个完全兼容的、可嵌入的高性能 Go MQTT v5（以及 v3.1.1）中间件/服务器。

Mochi MQTT 是一个[完全兼容 MQTT v5](https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html) 的可嵌入的中间件/服务器，完全使用 Go 语言编写，旨在用于遥测和物联网项目的开发。它可以作为独立的二进制文件使用，也可以嵌入到你自己的应用程序中作为库来使用，它经过精心设计以实现尽可能的轻量化和快速部署，同时我们也极为重视代码的质量和可维护性。

#### 什么是 MQTT?
MQTT 代表 MQ Telemetry Transport。它是一种发布/订阅、非常简单和轻量的消息传递协议，专为受限设备和低带宽、高延迟或不可靠网络设计而成（[了解更多](https://mqtt.org/faq)）。Mochi MQTT 实现了完整的 MQTT 协议的 5.0.0 版本。

#### Mochi-MQTT 特性
- 完全兼容 MQTT v5 功能，与 MQTT v3.1.1 和 v3.0.0 兼容：
    - MQTT v5 用户和数据包属性
    - 主题别名
    - 共享订阅
    - 订阅选项和订阅标识符(Identifiers)
    - 消息过期
    - 客户端会话过期
    - 发送和接收 QoS 流量控制配额(Flow Control Quotas)
    - 服务器端的断开连接和服务器端的数据包权限验证(Auth Packets)
    - 遗愿消息延迟间隔
    - 还有 Mochi MQTT v1 的所有原始 MQTT 功能，例如完全的 QoS（0,1,2）、$SYS 主题、保留消息等。
- 面向开发者：
    - 核心代码都已开放并可访问，以便开发者完全控制。
    - 功能丰富且灵活的基于钩子的接口系统，支持便捷的“插件”开发。
    - 使用特殊的内联客户端进行服务端的消息发布，也支持服务端伪装成现有的客户端。
- 高性能且稳定：
    - 基于经典前缀树 Trie 的主题-订阅模型。
    - 客户端特定的写入缓冲区，避免因读取速度慢或客户端不规范行为而产生的问题。
    - 通过所有 [Paho互操作性测试](https://github.com/eclipse/paho.mqtt.testing/tree/master/interoperability)（MQTT v5 和 MQTT v3）。
    - 超过一千多个经过仔细考虑的单元测试场景。
- 支持 TCP、Websocket（包括 SSL/TLS）和$SYS 服务状态监控。
- 内置 基于Redis、Badger 和 Bolt 的持久化（使用Hook钩子，你也可以自己创建）。
- 内置基于规则的认证和 ACL 权限管理（使用Hook钩子，你也可以自己创建）。

### 兼容性说明
由于 v5 规范与 MQTT 的早期版本存在重叠，因此服务器可以接受 v5 和 v3 客户端，但在连接了 v5 和 v3 客户端的情况下，为 v5 客户端提供的属性和功能将会对 v3 客户端进行降级处理（例如用户属性）。

对于 MQTT v3.0.0 和 v3.1.1 的支持被视为混合兼容性。在 v3 规范中没有明确限制的情况下，将使用更新的和以安全为首要考虑的 v5 规范 - 例如保留的消息(retained messages)的过期处理，进行中的消息(inflight messages)的过期处理、客户端过期处理以及QOS消息数量的限制等。

#### 版本更新时间
除非涉及关键问题，新版本通常在周末发布。

## 规划路线图
- 请[提出问题](https://github.com/mochi-mqtt/server/issues)来请求新功能或新的hook钩子接口！
- 集群支持。
- 统计度量支持。
- 配置文件支持（支持 Docker）。

## 快速开始
### 使用 Go 运行服务端
Mochi MQTT 可以作为独立的中间件使用。只需拉取此仓库代码，然后在 [cmd](cmd) 文件夹中运行 [cmd/main.go](cmd/main.go) ，默认将开启下面几个服务端口， tcp (:1883)、websocket (:1882) 和服务状态监控 (:8080) 。

```
cd cmd
go build -o mqtt && ./mqtt
```

### 使用 Docker

我们提供了一个简单的 Dockerfile，用于运行 cmd/main.go 中的 Websocket、TCP 和统计信息服务器：
```sh
docker build -t mochi:latest .
docker run -p 1883:1883 -p 1882:1882 -p 8080:8080 mochi:latest
```
更多关于 Docker 的支持正在[这里](https://github.com/orgs/mochi-mqtt/discussions/281#discussion-5544545)和[这里](https://github.com/orgs/mochi-mqtt/discussions/209)进行讨论。如果你有在这个场景下使用 Mochi-MQTT，也可以参与到讨论中来。

## 使用 Mochi MQTT 进行开发
### 将Mochi MQTT作为包导入使用
将 Mochi MQTT 作为一个包导入只需要几行代码即可开始使用。
``` go
import (
  "log"

  mqtt "github.com/mochi-mqtt/server/v2"
  "github.com/mochi-mqtt/server/v2/hooks/auth"
  "github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
  // 创建信号用于等待服务端关闭信号
  sigs := make(chan os.Signal, 1)
  done := make(chan bool, 1)
  signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
  go func() {
    <-sigs
    done <- true
  }()

  // 创建新的 MQTT 服务器。
  server := mqtt.New(nil)
  
  // 允许所有连接(权限)。
  _ = server.AddHook(new(auth.AllowHook), nil)
  
  // 在标1883端口上创建一个 TCP 服务端。
  tcp := listeners.NewTCP("t1", ":1883", nil)
  err := server.AddListener(tcp)
  if err != nil {
    log.Fatal(err)
  }
  

  go func() {
    err := server.Serve()
    if err != nil {
      log.Fatal(err)
    }
  }()

  // 服务端等待关闭信号
  <-done

  // 关闭服务端时需要做的一些清理工作
}
```

在 [examples](examples) 文件夹中可以找到更多使用不同配置运行服务端的示例。

# 网络监听器 Listeners

服务端内置了一些已经实现的网络监听Listeners，这些Listeners允许服务端接受不同协议的连接。当前的监听Listeners有这些：

| Listener                     | Usage                                                                                        |
|------------------------------|----------------------------------------------------------------------------------------------|
| listeners.NewTCP             | 一个 TCP 监听器，接收TCP连接                                                                   |
| listeners.NewUnixSock        | 一个 Unix 套接字监听器                                                                        |
| listeners.NewNet             | 一个 net.Listener 监听                                                                       |
| listeners.NewWebsocket       | 一个 Websocket 监听器                                                                        |
| listeners.NewHTTPStats       | 一个 HTTP $SYS 服务状态监听器                                                                 |
| listeners.NewHTTPHealthCheck | 一个 HTTP 健康检测监听器，用于为例如云基础设施提供健康检查响应                                   |

> 可以使用listeners.Listener接口开发新的监听器。如果有兴趣，你可以实现自己的Listener，如果你在此期间你有更好的建议或疑问，你可以[提交问题](https://github.com/mochi-mqtt/server/issues)给我们。 

可以在*listeners.Config 中配置TLS，传递给Listener使其支持TLS。
我们提供了一些示例，可以在 [示例](examples) 文件夹或 [cmd/main.go](cmd/main.go) 中找到。

### 服务端选项和功能

有许多可配置的选项(Options)可用于更改服务器的行为或限制对某些功能的访问。
```go
server := mqtt.New(&mqtt.Options{
  Capabilities: mqtt.Capabilities{
    MaximumSessionExpiryInterval: 3600,
    Compatibilities: mqtt.Compatibilities{
      ObscureNotAuthorized: true,
    },
  },
  ClientNetWriteBufferSize: 4096,
  ClientNetReadBufferSize: 4096,
  SysTopicResendInterval: 10,
  InlineClient: false,
})
```
请参考 mqtt.Options、mqtt.Capabilities 和 mqtt.Compatibilities 结构体，以查看完整的所有服务端选项。ClientNetWriteBufferSize 和 ClientNetReadBufferSize 可以根据你的需求配置调整每个客户端的内存使用状况。


## 事件钩子 Event Hooks 

服务端有一个通用的事件钩子(Event Hooks)系统，它允许开发人员在服务器和客户端生命周期的各个部分定制添加和修改服务端的功能。这些通用Hook钩子用于提供从认证(authentication)、持久性存储(persistent storage)到调试工具(debugging tools)等各种功能。

Hook钩子是可叠加的 - 你可以向服务器添加多个钩子(Hook)，它们将按添加的顺序运行。一些钩子(Hook)修改值，这些修改后的值将在所有钩子(Hooks)返回之前传递给后续的钩子(Hook)。

| 类型           | 导入包                                                                   | 描述                                                                       |
|----------------|--------------------------------------------------------------------------|----------------------------------------------------------------------------|
| 访问控制 | [mochi-mqtt/server/hooks/auth . AllowHook](hooks/auth/allow_all.go)      | AllowHook	允许所有客户端连接访问并读写所有主题。      | 
| 访问控制 | [mochi-mqtt/server/hooks/auth . Auth](hooks/auth/auth.go)                | 基于规则的访问权限控制。  | 
| 数据持久性    | [mochi-mqtt/server/hooks/storage/bolt](hooks/storage/bolt/bolt.go)       | 使用 [BoltDB](https://dbdb.io/db/boltdb) 进行持久性存储（已弃用）。 | 
| 数据持久性    | [mochi-mqtt/server/hooks/storage/badger](hooks/storage/badger/badger.go) | 使用 [BadgerDB](https://github.com/dgraph-io/badger) 进行持久性存储。   | 
| 数据持久性    | [mochi-mqtt/server/hooks/storage/redis](hooks/storage/redis/redis.go)    | 使用 [Redis](https://redis.io) 进行持久性存储。                         | 
| 调试跟踪      | [mochi-mqtt/server/hooks/debug](hooks/debug/debug.go)                    | 调试输出以查看数据包在服务端的链路追踪。   |

许多内部函数都已开放给开发者，你可以参考上述示例创建自己的Hook钩子。如果你有更好的关于Hook钩子方面的建议或者疑问，你可以[提交问题](https://github.com/mochi-mqtt/server/issues)给我们。                  | 

### 访问控制(Access Control)

#### 允许所有(Allow Hook)

默认情况下，Mochi MQTT 使用拒绝所有(DENY-ALL)的访问控制规则。要允许连接，必须使用访问控制钩子进行覆盖。其中最简单的钩子(Hook)是 auth.AllowAll 钩子(Hook)，它为所有连接、订阅和发布提供允许所有(ALLOW-ALL)的规则。这也是使用最简单的钩子：

```go
server := mqtt.New(nil)
_ = server.AddHook(new(auth.AllowHook), nil)
```

>如果你将服务器暴露在互联网或不受信任的网络上，请不要这样做 - 它真的应该仅用于开发、测试和调试。

#### 权限认证(Auth Ledger)

权限认证钩子(Auth Ledger hook)使用结构化的定义来制定访问规则。认证规则分为两种形式：身份规则（连接）和 ACL 权限规则（发布订阅）。

身份规则(Auth rules)有四个可选参数和一个是否允许参数：

| 参数 | 说明 | 
| -- | -- |
| Client | 客户端的客户端 ID |
| Username | 客户端的用户名 |
| Password | 客户端的密码 |
| Remote | 客户端的远程地址或 IP |
| Allow | true（允许此用户）或 false（拒绝此用户） | 

ACL rules have 3 optional criteria and an filter match:

ACL权限规则(ACL rules)有三个可选参数和一个过滤器匹配参数：
| 参数 | 说明 | 
| -- | -- |
| Client | 客户端的客户端 ID |
| Username | 客户端的用户名 |
| Remote | 客户端的远程地址或 IP |
| Filters | 用于匹配的过滤器数组 |

规则按索引顺序（0,1,2,3）处理，并在匹配到第一个规则时返回。请查看  [hooks/auth/ledger.go](hooks/auth/ledger.go) 的具体实现。

```go
server := mqtt.New(nil)
err := server.AddHook(new(auth.Hook), &auth.Options{
    Ledger: &auth.Ledger{
    Auth: auth.AuthRules{ // Auth 默认情况下禁止所有连接
      {Username: "peach", Password: "password1", Allow: true},
      {Username: "melon", Password: "password2", Allow: true},
      {Remote: "127.0.0.1:*", Allow: true},
      {Remote: "localhost:*", Allow: true},
    },
    ACL: auth.ACLRules{ // ACL 默认情况下允许所有连接
      {Remote: "127.0.0.1:*"}, // 本地用户允许所有连接
      {
        // 用户 melon 可以读取和写入自己的主题
        Username: "melon", Filters: auth.Filters{
          "melon/#":   auth.ReadWrite,
          "updates/#": auth.WriteOnly, // 可以写入 updates，但不能从其他人那里读取 updates
        },
      },
      {
        // 其他的客户端没有发布的权限
        Filters: auth.Filters{
          "#":         auth.ReadOnly,
          "updates/#": auth.Deny,
        },
      },
    },
  }
})
```

规则还可以存储为 JSON 或 YAML，并使用 Data 字段加载文件的二进制数据：

```go
err := server.AddHook(new(auth.Hook), &auth.Options{
    Data: data, // 从字节数组（文件二进制）读取规则：yaml 或 json
})
```
详细信息请参阅 [examples/auth/encoded/main.go](examples/auth/encoded/main.go)。

### 持久化存储

#### Redis

我们提供了一个基本的 Redis 存储Hook钩子，用于为服务端提供数据持久性。你可以将这个Redis的钩子添加到服务器中，Redis的一些参数也是可以配置的。这个钩子里使用 github.com/go-redis/redis/v8 这个库，可以通过 Options 来配置一些参数。

```go
err := server.AddHook(new(redis.Hook), &redis.Options{
  Options: &rv8.Options{
    Addr:     "localhost:6379", // Redis服务端地址
    Password: "",               // Redis服务端的密码
    DB:       0,                // Redis数据库的index
  },
})
if err != nil {
  log.Fatal(err)
}
```
有关 Redis 钩子的工作原理或如何使用它的更多信息，请参阅  [examples/persistence/redis/main.go](examples/persistence/redis/main.go) 或 [hooks/storage/redis](hooks/storage/redis) 。

#### Badger DB

如果您更喜欢基于文件的存储，还有一个 BadgerDB 存储钩子可用。它可以以与其他钩子大致相同的方式添加和配置（具有较少的选项）。

```go
err := server.AddHook(new(badger.Hook), &badger.Options{
  Path: badgerPath,
})
if err != nil {
  log.Fatal(err)
}
```

有关 Badger 钩子的工作原理或如何使用它的更多信息，请参阅 [examples/persistence/badger/main.go](examples/persistence/badger/main.go) 或 [hooks/storage/badger](hooks/storage/badger)。

还有一个 BoltDB 钩子，已被弃用，推荐使用 Badger，但如果你想使用它，请参考 [examples/persistence/bolt/main.go](examples/persistence/bolt/main.go)。

## 使用事件钩子 Event Hooks 进行开发

在服务端和客户端生命周期中，开发者可以使用各种Hook钩子增加对服务端或客户端的一些自定义的处理。
所有的钩子都定义在mqtt.Hook这个接口中了，可以在 [hooks.go](hooks.go) 中找到这些Hook钩子。

> 最灵活的事件钩子是 OnPacketRead、OnPacketEncode 和 OnPacketSent - 这些钩子可以用来控制和修改所有传入和传出的数据包。

| 钩子函数               | 说明                                                                                                                                                                                                                                                                                                      | 
|------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| OnStarted              | 在服务器成功启动时调 |用。                                                                                                                                                                                                                                                          |
| OnStopped              | 在服务器成功停止时调用。                                                                                                                                                                                                                                                           | 
| OnConnectAuthenticate  | 当用户尝试与服务器进行身份验证时调用。必须实现此方法来允许或拒绝对服务器的访问（请参阅 hooks/auth/allow_all 或 basic）。它可以在自定义Hook钩子中使用，以检查连接的用户是否与现有用户数据库中的用户匹配。如果允许访问，则返回 true。 |
| OnACLCheck             | 当用户尝试发布或订阅主题时调用，用来检测ACL规则。                                                                                                                                                                                                                          |
| OnSysInfoTick          | 当 $SYS 主题相关的消息被发布时调用。                                                                                                                                                                                                                                                      |
| OnConnect              |  当新客户端连接时调用，可能返回一个错误或错误码以中断客户端的连接。                                                                                                                                                                                        | 
| OnSessionEstablish     | 在新客户端连接并进行身份验证后，会立即调用此方法，并在会话建立和发送CONNACK之前立即调用。                                                                                                                                                                 |
| OnSessionEstablished   | 在新客户端成功建立会话（在OnConnect之后）时调用。                                                                                                                                                                                                                            | 
| OnDisconnect           | 当客户端因任何原因断开连接时调用。                                                                                                                                                                                                                                                      | 
| OnAuthPacket           | 当接收到认证数据包时调用。它旨在允许开发人员创建自己的 MQTT v5 认证数据包处理机制。在这里允许数据包的修改。                                                                                                                                       | 
| OnPacketRead           | 当从客户端接收到数据包时调用。允许对数据包进行修改。                                                                                                                                                                                                                               | 
| OnPacketEncode         | 在数据包被编码并发送给客户端之前立即调用。允许修改数据包。                                                                                                                                                                                                          | 
| OnPacketSent           | 在数据包已发送给客户端后调用。                                                                                                                                                                                                                                                 | 
| OnPacketProcessed      | 在数据包已接收并成功由服务端处理后调用。                                                                                                                                                                                                                             | 
| OnSubscribe            | 当客户端订阅一个或多个过滤器时调用。允许修改数据包。                                                                                                                                                                                                                       | 
| OnSubscribed           | 当客户端成功订阅一个或多个主题时调用。                                                                                                                                                                                                                                       | 
| OnSelectSubscribers    |  当订阅者已被关联到一个主题中，但在选择共享订阅的订阅者之前调用。允许接收者修改。                                                                                                                                                  | 
| OnUnsubscribe          | 当客户端取消订阅一个或多个主题时调用。允许包修改。                                                                                                                                                                                                                   | 
| OnUnsubscribed         | 当客户端成功取消订阅一个或多个主题时调用。                                                                                                                                                                                                                                   | 
| OnPublish              | 当客户端发布消息时调用。允许修改数据包。                                                                                                                                                                                                                                     | 
| OnPublished            | 当客户端向订阅者发布消息后调用。|                                        
| OnPublishDropped       |  消息传递给客户端之前消息已被丢弃，将调用此方法。 例如当客户端响应时间过长需要丢弃消息时。   | 
| OnRetainMessage        | 当消息被保留时调用。                                                                                                                                                                                                                                                              | 
| OnRetainPublished      | 当保留的消息被发布给客户端时调用。                                                                                                                                                                                                                                                  | 
| OnQosPublish           | Called when a publish packet with Qos >= 1 is issued to a subscriber.       当发出QoS >= 1 的消息给订阅者后调用。                                                                                                                                                                                                                               | 
| OnQosComplete          | 在消息的QoS流程走完之后调用。                                                                                                                                                                                                                                                 | 
| OnQosDropped           | 在消息的QoS流程未完成，同时消息到期时调用。                                                                                                                                                                                                                                                 | 
| OnPacketIDExhausted    | 当packet ids已经用完后，没有可用的id可再分配时调用。                                                                                                                                                                                                                                        | 
| OnWill                 | 当客户端断开连接并打算发布遗嘱消息时调用。允许修改数据包。                                                                                                                                                                                                          | 
| OnWillSent             | 遗嘱消息发送完成后被调用。                                                                                                                                                                                                                             | 
| OnClientExpired        | 在客户端会话已过期并应删除时调用。                                                                                                                                                                                                                                            | 
| OnRetainedExpired      | 在保留的消息已过期并应删除时调用。|                                                                                                                                                                                                                                         | 
| StoredClients          | 这个接口需要返回客户端列表，例如从持久化数据库中获取客户端列表。                                                                                                                                                                                          | 
| StoredSubscriptions    | 返回客户端的所有订阅，例如从持久化数据库中获取客户端的订阅列表。                            | 
| StoredInflightMessages | 返回正在传输中的消息（inflight messages），例如从持久化数据库中获取到还有哪些消息未完成传输。                                                                                                                                                                                                                                               | 
| StoredRetainedMessages | 返回保留的消息，例如从持久化数据库获取保留的消息。                                                                                                                                                                                                                                                   | 
| StoredSysInfo          | 返回存储的系统状态信息，例如从持久化数据库获取的系统状态信息。     | 

如果你自己实现一个持久化存储的Hook钩子，请参考现有的持久存储Hook钩子以获取灵感和借鉴。如果您正在构建一个身份验证Hook钩子，您将需要实现OnACLCheck 和 OnConnectAuthenticate这两个函数接口。

### 内联客户端 (v2.4.0+支持)

现在可以通过使用内联客户端功能直接在服务端上订阅和发布主题。内联客户端是内置在服务端中的特殊的客户端，可以在服务器配置中启用：

```go
server := mqtt.New(&mqtt.Options{
  InlineClient: true,
})
```
启用上述配置后，你将能够使用 server.Publish、server.Subscribe 和 server.Unsubscribe 方法来在服务端中直接发布和接收消息。

具体如何使用请参考 [direct examples](examples/direct/main.go) 。

#### 内联发布
要想在服务端中直接发布Publish一个消息，可以使用 `server.Publish()`方法。

```go
err := server.Publish("direct/publish", []byte("packet scheduled message"), false, 0)
```
> 在这种情况下，QoS级别只对订阅者有效，按照 MQTT v5 规范。

#### 内联订阅
要想在服务端中直接订阅一个主题，可以使用 `server.Subscribe()`方法并提供一个处理订阅消息的回调函数。内联订阅的 QoS默认都是0。如果您希望对相同的主题有多个回调，可以使用 MQTTv5 的 subscriptionId 属性进行区分。

```go
callbackFn := func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
    server.Log.Info("inline client received message from subscription", "client", cl.ID, "subscriptionId", sub.Identifier, "topic", pk.TopicName, "payload", string(pk.Payload))
}
server.Subscribe("direct/#", 1, callbackFn)
```

#### 取消内联订阅
如果您使用内联客户端订阅了某个主题，如果需要取消订阅。您可以使用 `server.Unsubscribe()` 方法取消内联订阅：

```go
server.Unsubscribe("direct/#", 1)
```

### 注入数据包

如果你想要更多的服务端控制，或者想要设置特定的MQTT v5属性或其他属性，你可以选择指定的客户端创建自己的发布包(publish packets)。这种方法允许你将MQTT数据包(packets)直接注入到运行中的服务端，相当于服务端直接自己模拟接收到了某个客户端的数据包。

数据包注入可用于任何MQTT数据包，包括ping请求、订阅等。你可以获取客户端的详细信息，因此你甚至可以直接在服务端模拟某个在线的客户端，发布一个数据包。

大多数情况下，您可能希望使用上面描述的内联客户端，因为它具有独特的特权：它可以绕过所有ACL和主题验证检查，这意味着它甚至可以发布到$SYS主题。你也可以自己从头开始制定一个自己的内联客户端，它将与内置的内联客户端行为相同。

```go
cl := server.NewClient(nil, "local", "inline", true)
server.InjectPacket(cl, packets.Packet{
  FixedHeader: packets.FixedHeader{
    Type: packets.Publish,
  },
  TopicName: "direct/publish",
  Payload: []byte("scheduled message"),
})
```

> MQTT数据包仍然需要正确结构，所以请参考[测试用例中数据包的定义](packets/tpackets.go) 和 [MQTTv5规范](https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html) 以获取一些帮助。

具体如何使用请参考 [hooks example](examples/hooks/main.go) 。


### Testing
#### Unit Tests
Mochi MQTT tests over a thousand scenarios with thoughtfully hand written unit tests to ensure each function does exactly what we expect. You can run the tests using go:
```
go run --cover ./...
```

#### Paho Interoperability Test
You can check the broker against the [Paho Interoperability Test](https://github.com/eclipse/paho.mqtt.testing/tree/master/interoperability) by starting the broker using `examples/paho/main.go`, and then running the mqtt v5 and v3 tests with `python3 client_test5.py` from the _interoperability_ folder. 

> Note that there are currently a number of outstanding issues regarding false negatives in the paho suite, and as such, certain compatibility modes are enabled in the `paho/main.go` example.


## Performance Benchmarks
Mochi MQTT performance is comparable with popular brokers such as Mosquitto, EMQX, and others.

Performance benchmarks were tested using [MQTT-Stresser](https://github.com/inovex/mqtt-stresser) on a Apple Macbook Air M2, using `cmd/main.go` default settings. Taking into account bursts of high and low throughput, the median scores are the most useful. Higher is better.

> The values presented in the benchmark are not representative of true messages per second throughput. They rely on an unusual calculation by mqtt-stresser, but are usable as they are consistent across all brokers.
> Benchmarks are provided as a general performance expectation guideline only. Comparisons are performed using out-of-the-box default configurations.

`mqtt-stresser -broker tcp://localhost:1883 -num-clients=2 -num-messages=10000`
| Broker            | publish fastest | median | slowest | receive fastest | median | slowest | 
| --                | --             | --   | --   | --             | --   | --   |
| Mochi v2.2.10      | 124,772 | 125,456 | 124,614 | 314,461 | 313,186 | 311,910 |
| [Mosquitto v2.0.15](https://github.com/eclipse/mosquitto) | 155,920 | 155,919 | 155,918 | 185,485 | 185,097 | 184,709 |
| [EMQX v5.0.11](https://github.com/emqx/emqx)      | 156,945 | 156,257 | 155,568 | 17,918 | 17,783 | 17,649 |
| [Rumqtt v0.21.0](https://github.com/bytebeamio/rumqtt) | 112,208 | 108,480 | 104,753 | 135,784 | 126,446 | 117,108 |

`mqtt-stresser -broker tcp://localhost:1883 -num-clients=10 -num-messages=10000`
| Broker            | publish fastest | median | slowest | receive fastest | median | slowest | 
| --                | --             | --   | --   | --             | --   | --   |
| Mochi v2.2.10      | 41,825 | 31,663| 23,008 | 144,058 | 65,903 | 37,618 |
| Mosquitto v2.0.15 | 42,729 | 38,633 | 29,879 | 23,241 | 19,714 | 18,806 |
| EMQX v5.0.11      | 21,553 | 17,418 | 14,356 | 4,257 | 3,980 | 3,756 |
| Rumqtt v0.21.0    | 42,213 | 23,153 | 20,814 | 49,465 | 36,626 | 19,283 |

Million Message Challenge (hit the server with 1 million messages immediately):

`mqtt-stresser -broker tcp://localhost:1883 -num-clients=100 -num-messages=10000`
| Broker            | publish fastest | median | slowest | receive fastest | median | slowest | 
| --                | --             | --   | --   | --             | --   | --   |
| Mochi v2.2.10     | 13,532 | 4,425 | 2,344 | 52,120 | 7,274 | 2,701 |
| Mosquitto v2.0.15 | 3,826 | 3,395 | 3,032 | 1,200 | 1,150 | 1,118 |
| EMQX v5.0.11      | 4,086 | 2,432 | 2,274 | 434 | 333 | 311 |
| Rumqtt v0.21.0    | 78,972 | 5,047 | 3,804 | 4,286 | 3,249 | 2,027 |

> Not sure what's going on with EMQX here, perhaps the docker out-of-the-box settings are not optimal, so take it with a pinch of salt as we know for a fact it's a solid piece of software.

## Contribution Guidelines
Contributions and feedback are both welcomed and encouraged! [Open an issue](https://github.com/mochi-mqtt/server/issues) to report a bug, ask a question, or make a feature request. If you open a pull request, please try to follow the following guidelines:
- Try to maintain test coverage where reasonably possible.
- Clearly state what the PR does and why.
- Please remember to add your SPDX FileContributor tag to files where you have made a meaningful contribution.

[SPDX Annotations](https://spdx.dev) are used to clearly indicate the license, copyright, and contributions of each file in a machine-readable format. If you are adding a new file to the repository, please ensure it has the following SPDX header:
```go
// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt
// SPDX-FileContributor: Your name or alias <optional@email.address>

package name
```

Please ensure to add a new `SPDX-FileContributor` line for each contributor to the file. Refer to other files for examples. Please remember to do this, your contributions to this project are valuable and appreciated - it's important to receive credit! 

## Stargazers over time 🥰
[![Stargazers over time](https://starchart.cc/mochi-mqtt/server.svg)](https://starchart.cc/mochi-mqtt/server)
Are you using Mochi MQTT in a project? [Let us know!](https://github.com/mochi-mqtt/server/issues)

