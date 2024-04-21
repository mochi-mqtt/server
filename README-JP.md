# Mochi-MQTT Server

<p align="center">
    
![build status](https://github.com/mochi-mqtt/server/actions/workflows/build.yml/badge.svg) 
[![Coverage Status](https://coveralls.io/repos/github/mochi-mqtt/server/badge.svg?branch=master&v2)](https://coveralls.io/github/mochi-mqtt/server?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/mochi-mqtt/server)](https://goreportcard.com/report/github.com/mochi-mqtt/server/v2)
[![Go Reference](https://pkg.go.dev/badge/github.com/mochi-mqtt/server.svg)](https://pkg.go.dev/github.com/mochi-mqtt/server/v2)
[![contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat)](https://github.com/mochi-mqtt/server/issues)

</p>

[English](README.md) | [简体中文](README-CN.md) | [日本語](README-JP.md) | [Translators Wanted!](https://github.com/orgs/mochi-mqtt/discussions/310)

🎆 **mochi-co/mqtt は新しい mochi-mqtt organisation の一部です.** [このページをお読みください](https://github.com/orgs/mochi-mqtt/discussions/271)


### Mochi-MQTTは MQTT v5 (と v3.1.1)に完全に準拠しているアプリケーションに組み込み可能なハイパフォーマンスなbroker/serverです.

Mochi MQTT は Goで書かれたMQTT v5に完全に[準拠](https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html)しているMQTTブローカーで、IoTプロジェクトやテレメトリの開発プロジェクト向けに設計されています。 スタンドアロンのバイナリで使ったり、アプリケーションにライブラリとして組み込むことができ、プロジェクトのメンテナンス性と品質を確保できるように配慮しながら、 軽量で可能な限り速く動作するように設計されています。

#### MQTTとは?
MQTT は [MQ Telemetry Transport](https://en.wikipedia.org/wiki/MQTT)を意味します。 Pub/Sub型のシンプルで軽量なメッセージプロトコルで、低帯域、高遅延、不安定なネットワーク下での制約を考慮して設計されています([MQTTについて詳しくはこちら](https://mqtt.org/faq))。 Mochi MQTTはMQTTプロトコルv5.0.0に完全準拠した実装をしています。

#### Mochi-MQTTのもつ機能

- MQTTv5への完全な準拠とMQTT v3.1.1 および v3.0.0 との互換性:
    - MQTT v5で拡張されたユーザープロパティ
    - トピック・エイリアス
    - 共有サブスクリプション
    - サブスクリプションオプションとサブスクリプションID
    - メッセージの有効期限
    - クライアントセッション
    - 送受信QoSフロー制御クォータ
    - サーバサイド切断と認証パケット
    - Will遅延間隔
    - 上記に加えてQoS(0,1,2)、$SYSトピック、retain機能などすべてのMQTT v1の特徴を持ちます
- Developer-centric:
    - 開発者が制御できるように、ほとんどのコアブローカーのコードをエクスポートにしてアクセスできるようにしました。
    - フル機能で柔軟なフックベースのインターフェイスにすることで簡単に'プラグイン'を開発できるようにしました。
    - 特別なインラインクライアントを利用することでパケットインジェクションを行うか、既存のクライアントとしてマスカレードすることができます。
- パフォーマンスと安定性:
    - 古典的なツリーベースのトピックサブスクリプションモデル
    - クライアント固有に書き込みバッファーをもたせることにより、読み込みの遅さや不規則なクライアントの挙動の問題を回避しています。
    - MQTT v5 and MQTT v3のすべての[Paho互換性テスト](https://github.com/eclipse/paho.mqtt.testing/tree/master/interoperability)をpassしています。
    - 慎重に検討された多くのユニットテストシナリオでテストされています。
- TCP, Websocket (SSL/TLSを含む), $SYSのダッシュボードリスナー
- フックを利用した保存機能としてRedis, Badger, Boltを使うことができます（自作のHookも可能です）。
- フックを利用したルールベース認証機能とアクセス制御リストLedgerを使うことができます（自作のHookも可能です）。

### 互換性に関する注意事項
MQTTv5とそれ以前との互換性から、サーバーはv5とv3両方のクライアントを受け入れることができますが、v5とv3のクライアントが接続された場合はv5でクライアント向けの特徴と機能はv3クライアントにダウングレードされます（ユーザープロパティなど）。
MQTT v3.0.0 と v3.1.1 のサポートはハイブリッド互換性があるとみなされます。それはv3と仕様に制限されていない場合、例えば、送信メッセージ、保持メッセージの有効期限とQoSフロー制御制限などについては、よりモダンで安全なv5の動作が使用されます

#### リリースされる時期について
クリティカルなイシュー出ない限り、新しいリリースがされるのは週末です。

## Roadmap
- 新しい特徴やイベントフックのリクエストは [open an issue](https://github.com/mochi-mqtt/server/issues) へ！
- クラスターのサポート
- メトリックスサポートの強化
- ファイルベースの設定(Dockerイメージのサポート)

## Quick Start
### GoでのBrokerの動かし方
Mochi MQTTはスタンドアロンのブローカーとして使うことができます。単純にこのレポジトリーをチェックアウトして、[cmd/main.go](cmd/main.go) を起動すると内部の [cmd](cmd) フォルダのエントリポイントにしてtcp (:1883), websocket (:1882),  dashboard (:8080)のポートを外部にEXPOSEします。

```
cd cmd
go build -o mqtt && ./mqtt
```

### Dockerで利用する
Dockerレポジトリの [official Mochi MQTT image](https://hub.docker.com/r/mochimqtt/server) から Pullして起動することができます。

```sh
docker pull mochimqtt/server
or
docker run mochimqtt/server
```

これは実装途中です。[file-based configuration](https://github.com/orgs/mochi-mqtt/projects/2) は、この実装をよりよくサポートするために開発中です。
より実質的なdockerのサポートが議論されています。_Docker環境で使っている方は是非この議論に参加してください。_ [ここ](https://github.com/orgs/mochi-mqtt/discussions/281#discussion-5544545) や [ここ](https://github.com/orgs/mochi-mqtt/discussions/209)。

[cmd/main.go](cmd/main.go)の Websocket, TCP, Statsサーバを実行するために、シンプルなDockerfileが提供されます。 


```sh
docker build -t mochi:latest .
docker run -p 1883:1883 -p 1882:1882 -p 8080:8080 mochi:latest
```

## Mochi MQTTを使って開発するには
### パッケージをインポート
Mochi MQTTをパッケージとしてインポートするにはほんの数行のコードで始めることができます。
``` go
import (
  "log"

  mqtt "github.com/mochi-mqtt/server/v2"
  "github.com/mochi-mqtt/server/v2/hooks/auth"
  "github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
  // Create signals channel to run server until interrupted
  sigs := make(chan os.Signal, 1)
  done := make(chan bool, 1)
  signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
  go func() {
    <-sigs
    done <- true
  }()

  // Create the new MQTT Server.
  server := mqtt.New(nil)
  
  // Allow all connections.
  _ = server.AddHook(new(auth.AllowHook), nil)
  
  // Create a TCP listener on a standard port.
  tcp := listeners.NewTCP(listeners.Config{ID: "t1", Address: ":1883"})
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

  // Run server until interrupted
  <-done

  // Cleanup
}
```

ブローカーの動作例は [examples](examples)フォルダにあります。

#### Network Listeners
サーバは様々なプロトコルのコネクションのリスナーに対応しています。現在の対応リスナーは、

| Listener                     | Usage                                                                                        |
|------------------------------|----------------------------------------------------------------------------------------------|
| listeners.NewTCP             | TCPリスナー                                                                              |
| listeners.NewUnixSock        | Unixソケットリスナー                                                                      |
| listeners.NewNet             | net.Listenerリスナー                                                                      |
| listeners.NewWebsocket       | Websocketリスナー                                                                         |
| listeners.NewHTTPStats       | HTTP $SYSダッシュボード                                                                 |
| listeners.NewHTTPHealthCheck | ヘルスチェック応答を提供するためのHTTPヘルスチェックリスナー(クラウドインフラ) |

> 新しいリスナーを開発するためには `listeners.Listener` を使ってください。使ったら是非教えてください！

TLSを設定するには`*listeners.Config`を渡すことができます。

[examples](examples) フォルダと [cmd/main.go](cmd/main.go)に使用例があります。


## 設定できるオプションと機能
たくさんのオプションが利用可能です。サーバーの動作を変更したり、特定の機能へのアクセスを制限することができます。

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

mqtt.Options、mqtt.Capabilities、mqtt.Compatibilitiesの構造体はオプションの理解に役立ちます。
必要に応じて`ClientNetWriteBufferSize`と`ClientNetReadBufferSize`はクライアントの使用するメモリに合わせて設定できます。

### デフォルト設定に関する注意事項

いくつかのデフォルトの設定を決める際にいくつかの決定がなされましたのでここに記しておきます:
- デフォルトとして、敵対的なネットワーク上のDoSアタックにさらされるのを防ぐために `server.Options.Capabilities.MaximumMessageExpiryInterval`は86400 (24時間)に、とセットされています。有効期限を無限にすると、保持、送信メッセージが無限に蓄積されるからです。もし信頼できる環境であったり、より大きな保存期間が可能であれば、この設定はオーバーライドできます（`0` を設定すると有効期限はなくなります。）

## Event Hooks 
ユニバーサルイベントフックシステムは、開発者にサーバとクライアントの様々なライフサイクルをフックすることができ、ブローカーの機能を追加/変更することができます。それらのユニバーサルフックは認証、永続ストレージ、デバッグツールなど、あらゆるものに使用されています。
フックは複数重ねることができ、サーバに複数のフックを設定することができます。それらは追加した順番に動作します。いくつかのフックは値を変えて、その値は動作コードに返される前にあとに続くフックに渡されます。


| Type           | Import                                                                   | Info                                                                       |
|----------------|--------------------------------------------------------------------------|----------------------------------------------------------------------------|
| Access Control | [mochi-mqtt/server/hooks/auth . AllowHook](hooks/auth/allow_all.go)      | すべてのトピックに対しての読み書きをすべてのクライアントに対して許可します。     | 
| Access Control | [mochi-mqtt/server/hooks/auth . Auth](hooks/auth/auth.go)                | ルールベースのアクセスコントロール台帳です。                                         | 
| Persistence    | [mochi-mqtt/server/hooks/storage/bolt](hooks/storage/bolt/bolt.go)       |  [BoltDB](https://dbdb.io/db/boltdb) を使った永続ストレージ (非推奨). | 
| Persistence    | [mochi-mqtt/server/hooks/storage/badger](hooks/storage/badger/badger.go) | [BadgerDB](https://github.com/dgraph-io/badger)を使った永続ストレージ  | 
| Persistence    | [mochi-mqtt/server/hooks/storage/redis](hooks/storage/redis/redis.go)    | [Redis](https://redis.io)を使った永続ストレージ                   | 
| Debugging      | [mochi-mqtt/server/hooks/debug](hooks/debug/debug.go)                    | パケットフローを可視化するデバッグ用のフック                       | 

たくさんの内部関数が開発者に公開されています、なので、上記の例を使って自分でフックを作ることができます。もし作ったら是非[Open an issue](https://github.com/mochi-mqtt/server/issues)に投稿して教えてください！

### アクセスコントロール
#### Allow Hook
デフォルトで、Mochi MQTTはアクセスコントロールルールにDENY-ALLを使用しています。コネクションを許可するためには、アクセスコントロールフックを上書きする必要があります。一番単純なのは`auth.AllowAll`フックで、ALLOW-ALLルールがすべてのコネクション、サブスクリプション、パブリッシュに適用されます。使い方は下記のようにするだけです: 

```go
server := mqtt.New(nil)
_ = server.AddHook(new(auth.AllowHook), nil)
```

> もしインターネットや信頼できないネットワークにさらされる場合は行わないでください。これは開発・テスト・デバッグ用途のみであるべきです。

#### Auth Ledger
Auth Ledgerは構造体で定義したアクセスルールの洗練された仕組みを提供します。Auth Ledgerルール２つの形式から成ります、認証ルール(コネクション)とACLルール(パブリッシュ、サブスクライブ)です。

認証ルールは4つのクライテリアとアサーションフラグがあります: 
| Criteria | Usage | 
| -- | -- |
| Client | 接続クライアントのID |
| Username | 接続クライアントのユーザー名 |
| Password | 接続クライアントのパスワード |
| Remote | クライアントのリモートアドレスもしくはIP |
| Allow | true(このユーザーを許可する)もしくはfalse(このユーザを拒否する) | 

アクセスコントロールルールは3つのクライテリアとフィルターマッチがあります:
| Criteria | Usage | 
| -- | -- |
| Client | 接続クライアントのID |
| Username | 接続クライアントのユーザー名 |
| Remote | クライアントのリモートアドレスもしくはIP |
| Filters | 合致するフィルターの配列 |

ルールはインデックス順(0,1,2,3)に処理され、はじめに合致したルールが適用されます。 [hooks/auth/ledger.go](hooks/auth/ledger.go) の構造体を見てください。


```go
server := mqtt.New(nil)
err := server.AddHook(new(auth.Hook), &auth.Options{
    Ledger: &auth.Ledger{
    Auth: auth.AuthRules{ // Auth disallows all by default
      {Username: "peach", Password: "password1", Allow: true},
      {Username: "melon", Password: "password2", Allow: true},
      {Remote: "127.0.0.1:*", Allow: true},
      {Remote: "localhost:*", Allow: true},
    },
    ACL: auth.ACLRules{ // ACL allows all by default
      {Remote: "127.0.0.1:*"}, // local superuser allow all
      {
        // user melon can read and write to their own topic
        Username: "melon", Filters: auth.Filters{
          "melon/#":   auth.ReadWrite,
          "updates/#": auth.WriteOnly, // can write to updates, but can't read updates from others
        },
      },
      {
        // Otherwise, no clients have publishing permissions
        Filters: auth.Filters{
          "#":         auth.ReadOnly,
          "updates/#": auth.Deny,
        },
      },
    },
  }
})
```

ledgeはデータフィールドを使用してJSONもしくはYAML形式で保存したものを使用することもできます。
```go
err := server.AddHook(new(auth.Hook), &auth.Options{
    Data: data, // build ledger from byte slice: yaml or json
})
```
より詳しくは[examples/auth/encoded/main.go](examples/auth/encoded/main.go)を見てください。

### 永続ストレージ
#### Redis
ブローカーに永続性を提供する基本的な Redis ストレージフックが利用可能です。他のフックと同じ方法で、いくつかのオプションを使用してサーバーに追加できます。それはフック内部で github.com/go-redis/redis/v8 を使用し、Optionsの値で詳しい設定を行うことができます。
```go
err := server.AddHook(new(redis.Hook), &redis.Options{
  Options: &rv8.Options{
    Addr:     "localhost:6379", // default redis address
    Password: "",               // your password
    DB:       0,                // your redis db
  },
})
if err != nil {
  log.Fatal(err)
}
```
Redisフックがどのように動くか、どのように使用するかについての詳しくは、[examples/persistence/redis/main.go](examples/persistence/redis/main.go) か [hooks/storage/redis](hooks/storage/redis) のソースコードを見てください。

#### Badger DB
もしファイルベースのストレージのほうが適しているのであれば、BadgerDBストレージも使用することができます。それもまた、他のフックと同様に追加、設定することができます（オプションは若干少ないです）。

```go
err := server.AddHook(new(badger.Hook), &badger.Options{
  Path: badgerPath,
})
if err != nil {
  log.Fatal(err)
}
```

badgerフックがどのように動くか、どのように使用するかについての詳しくは、[examples/persistence/badger/main.go](examples/persistence/badger/main.go) か [hooks/storage/badger](hooks/storage/badger) のソースコードを見てください。

BoltDBフックはBadgerに代わって非推奨となりましたが、もし必要ならば [examples/persistence/bolt/main.go](examples/persistence/bolt/main.go)をチェックしてください。

## イベントフックを利用した開発

ブローカーとクライアントのライフサイクルに関わるたくさんのフックが利用できます。
そのすべてのフックと`mqtt.Hook`インターフェイスの関数シグネチャは[hooks.go](hooks.go)に記載されています。

> もっと柔軟なイベントフックはOnPacketRead、OnPacketEncodeとOnPacketSentです。それらは、すべての流入パケットと流出パケットをコントロール及び変更に使用されるフックです。


| Function               | Usage                                                                                                                                                                                                                                                                                                      | 
|------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| OnStarted              | サーバーが正常にスタートした際に呼ばれます。                                                                                                                                                                                                                                                         |
| OnStopped              | サーバーが正常に終了した際に呼ばれます。                                                                                                                                                                                                                                                           | 
| OnConnectAuthenticate  | ユーザーがサーバと認証を試みた際に呼ばれます。このメソッドはサーバーへのアクセス許可もしくは拒否するためには必ず使用する必要があります（hooks/auth/allow_all or basicを見てください）。これは、データベースにユーザーが存在するか照合してチェックするカスタムフックに利用できます。許可する場合はtrueを返す実装をします。|
| OnACLCheck             | ユーザーがあるトピックフィルタにpublishかsubscribeした際に呼ばれます。上と同様です                                                   |
| OnSysInfoTick          | $SYSトピック値がpublishされた場合に呼ばれます。                                                                                                                                      |
| OnConnect              | 新しいクライアントが接続した際によばれます、エラーかパケットコードを返して切断する場合があります。                                                                                                                                                 | 
| OnSessionEstablish     | 新しいクライアントが接続された後すぐ、セッションが確立されてCONNACKが送信される前に呼ばれます。                                                                                                                              |
| OnSessionEstablished   | 新しいクライアントがセッションを確立した際(OnConnectの後)に呼ばれます。                                                                                                                                               | 
| OnDisconnect           | クライアントが何らかの理由で切断された場合に呼ばれます。                                                                                                                                                                                                                             | 
| OnAuthPacket           | 認証パケットを受け取ったときに呼ばれます。これは開発者にmqtt v5の認証パケットを取り扱う仕組みを作成すること意図しています。パケットを変更することができます。                                                                                                                                        | 
| OnPacketRead           | クライアントからパケットを受け取った際に呼ばれます。パケットを変更することができます。                                                                                                                                                                                                                                | 
| OnPacketEncode         | エンコードされたパケットがクライアントに送信する直前に呼ばれます。パケットを変更することができます。                                                                                                   | 
| OnPacketSent           | クライアントにパケットが送信された際に呼ばれます。                                                                                                                                                                                                                                                           | 
| OnPacketProcessed      | パケットが届いてブローカーが正しく処理できた場合に呼ばれます。                                                                                                                                                                                                                      | 
| OnSubscribe            | クライアントが１つ以上のフィルタをsubscribeした場合に呼ばれます。パケットの変更ができます。                                                                                                                                                                                                              | 
| OnSubscribed           | クライアントが１つ以上のフィルタをsubscribeに成功した場合に呼ばれます。                                                                                                                                                                                                                                       | 
| OnSelectSubscribers    | サブスクライバーがトピックに収集されたとき、共有サブスクライバーが選択される前に呼ばれる。受信者は変更可能。                                                                                                                                               | 
| OnUnsubscribe          | １つ以上のあんサブスクライブが呼ばれた場合。パケットの変更は可能。                                                                                                                                                                                                                 | 
| OnUnsubscribed         | クライアントが正常に1つ以上のトピックフィルタをサブスクライブ解除した場合。                                                                                                                                                                                                                                   | 
| OnPublish              | クライアントがメッセージをパブリッシュした場合。パケットの変更は可能。                                                                                                                                                   | 
| OnPublished            | クライアントがサブスクライバーにメッセージをパブリッシュし終わった場合。                                                                                                                                                                                                                                      | 
| OnPublishDropped       | あるクライアントが反応に時間がかかった場合等のようにクライアントに到達する前にメッセージが失われた場合に呼ばれる。                                                                                                                                                                                         | 
| OnRetainMessage        | パブリッシュされたメッセージが保持された場合に呼ばれる。                                                                                                                                                                                                                                                    | 
| OnRetainPublished      | 保持されたメッセージがクライアントに到達した場合に呼ばれる。                                                                                                                                                                                                                                          | 
| OnQosPublish           | QoSが1以上のパケットがサブスクライバーに発行された場合。                                                                                                                                                                                | 
| OnQosComplete          | そのメッセージQoSフローが完了した場合に呼ばれる。                                                                                                                                                                                                             | 
| OnQosDropped           | インフライトメッセージが完了前に期限切れになった場合に呼ばれる。                                                                                                                                                                                                                                                | 
| OnPacketIDExhausted    |  クライアントがパケットに割り当てるIDが枯渇した場合に呼ばれる。                                                                                                                                                                                                                                             | 
| OnWill                 | クライアントが切断し、WILLメッセージを発行しようとした場合に呼ばれる。パケットの変更が可能。                                                                                                                                                                                        | 
| OnWillSent             | LWTメッセージが切断されたクライアントから発行された場合に呼ばれる                                                                                                                                                                                                                                   | 
| OnClientExpired        | クライアントセッションが期限切れで削除するべき場合に呼ばれる。                                                                                                                                                                                                                                 | 
| OnRetainedExpired      | 保持メッセージが期限切れで削除すべき場合に呼ばれる。                                                                                                                                                                               | 
| StoredClients          | クライアントを返す。例えば永続ストレージから。                                                                                                                                                                                                                                                             | 
| StoredSubscriptions    | クライアントのサブスクリプションを返す。例えば永続ストレージから。                                                                                                                                                                                                                                            | 
| StoredInflightMessages | インフライトメッセージを返す。例えば永続ストレージから。                                                                                                                                                                                                                                                | 
| StoredRetainedMessages | 保持されたメッセージを返す。例えば永続ストレージから。                                                                                                                                                                                                                                                 | 
| StoredSysInfo          | システム情報の値を返す。例えば永続ストレージから。                                                                                                                                                                                                                                        | 

もし永続ストレージフックを作成しようとしているのであれば、すでに存在する永続的なフックを見てインスピレーションとどのようなパターンがあるか見てみてください。もし認証フックを作成しようとしているのであれば、`OnACLCheck`と`OnConnectAuthenticate`が役立つでしょう。

### Inline Client (v2.4.0+)
トピックに対して埋め込まれたコードから直接サブスクライブとパブリッシュできます。そうするには`inline client`機能を使うことができます。インラインクライアント機能はサーバの一部として組み込まれているクライアントでサーバーのオプションとしてEnableにできます。
```go
server := mqtt.New(&mqtt.Options{
  InlineClient: true,
})
```
Enableにすると、`server.Publish`, `server.Subscribe`, `server.Unsubscribe`のメソッドを利用できて、ブローカーから直接メッセージを送受信できます。
> 実際の使用例は[direct examples](examples/direct/main.go)を見てください。

#### Inline Publish
組み込まれたアプリケーションからメッセージをパブリッシュするには`server.Publish(topic string, payload []byte, retain bool, qos byte) error`メソッドを利用します。

```go
err := server.Publish("direct/publish", []byte("packet scheduled message"), false, 0)
```
> このケースでのQoSはサブスクライバーに設定できる上限でしか使用されません。これはMQTTv5の仕様に従っています。

#### Inline Subscribe
組み込まれたアプリケーション内部からトピックフィルタをサブスクライブするには、`server.Subscribe(filter string, subscriptionId int, handler InlineSubFn) error`メソッドがコールバックも含めて使用できます。
インラインサブスクリプションではQoS0のみが適用されます。もし複数のコールバックを同じフィルタに設定したい場合は、MQTTv5の`subscriptionId`のプロパティがその区別に使用できます。

```go
callbackFn := func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
    server.Log.Info("inline client received message from subscription", "client", cl.ID, "subscriptionId", sub.Identifier, "topic", pk.TopicName, "payload", string(pk.Payload))
}
server.Subscribe("direct/#", 1, callbackFn)
```

#### Inline Unsubscribe
インラインクライアントでサブスクリプション解除をしたい場合は、`server.Unsubscribe(filter string, subscriptionId int) error` メソッドで行うことができます。

```go
server.Unsubscribe("direct/#", 1)
```

### Packet Injection
もし、より制御したい場合や、特定のMQTTv5のプロパティやその他の値をセットしたい場合は、クライアントからのパブリッシュパケットを自ら作成することができます。この方法は単なるパブリッシュではなく、MQTTパケットをまるで特定のクライアントから受け取ったかのようにランタイムに直接インジェクションすることができます。

このパケットインジェクションは例えばPING ReqやサブスクリプションなどのどんなMQTTパケットでも使用できます。そしてクライアントの構造体とメソッドはエクスポートされているので、(もし、非常にカスタマイズ性の高い要求がある場合には)まるで接続されたクライアントに代わってパケットをインジェクションすることさえできます。

たいていの場合は上記のインラインクライアントを使用するのが良いでしょう、それはACLとトピックバリデーションをバイパスできる特権があるからです。これは$SYSトピックにさえパブリッシュできることも意味します。ビルトインのクライアントと同様に振る舞うインラインクライアントを作成できます。

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

> MQTTのパケットは正しく構成する必要があり、なので[the test packets catalogue](packets/tpackets.go)と[MQTTv5 Specification](https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html)を参照してください。

この機能の動作を確認するには[hooks example](examples/hooks/main.go) を見てください。


### Testing
#### ユニットテスト
それぞれの関数が期待通りの動作をするように考えられてMochi MQTTテストが作成されています。テストを走らせるには:
```
go run --cover ./...
```

#### Paho相互運用性テスト
`examples/paho/main.go`を使用してブローカーを起動し、_interoperability_フォルダの`python3 client_test5.py`のmqttv5とv3のテストを実行することで、[Paho Interoperability Test](https://github.com/eclipse/paho.mqtt.testing/tree/master/interoperability)を確認することができます。

> pahoスイートには現在は何個かの偽陰性に関わるissueがあるので、`paho/main.go`の例ではいくつかの互換性モードがオンになっていることに注意してください。



## ベンチマーク
Mochi MQTTのパフォーマンスはMosquitto、EMQX、その他などの有名なブローカーに匹敵します。

ベンチマークはApple Macbook Air M2上で[MQTT-Stresser](https://github.com/inovex/mqtt-stresser)、セッティングとして`cmd/main.go`のデフォルト設定を使用しています。高スループットと低スループットのバーストを考慮すると、中央値のスコアが最も信頼できます。この値は高いほど良いです。

> ベンチマークの値は1秒あたりのメッセージ数のスループットのそのものを表しているわけではありません。これは、mqtt-stresserによる固有の計算に依存するものではありますが、すべてのブローカーに渡って一貫性のある値として利用しています。
> ベンチマークは一般的なパフォーマンス予測ガイドラインとしてのみ提供されます。比較はそのまま使用したデフォルトの設定値で実行しています。

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

100万メッセージ試験 (100 万メッセージを一斉にサーバーに送信します):

`mqtt-stresser -broker tcp://localhost:1883 -num-clients=100 -num-messages=10000`
| Broker            | publish fastest | median | slowest | receive fastest | median | slowest | 
| --                | --             | --   | --   | --             | --   | --   |
| Mochi v2.2.10     | 13,532 | 4,425 | 2,344 | 52,120 | 7,274 | 2,701 |
| Mosquitto v2.0.15 | 3,826 | 3,395 | 3,032 | 1,200 | 1,150 | 1,118 |
| EMQX v5.0.11      | 4,086 | 2,432 | 2,274 | 434 | 333 | 311 |
| Rumqtt v0.21.0    | 78,972 | 5,047 | 3,804 | 4,286 | 3,249 | 2,027 |

> EMQXのここでの結果は何が起きているのかわかりませんが、おそらくDockerのそのままの設定が最適ではなかったのでしょう、なので、この結果はソフトウェアのひとつの側面にしか過ぎないと捉えてください。


## Contribution Guidelines
コントリビューションとフィードバックは両方とも歓迎しています![Open an issue](https://github.com/mochi-mqtt/server/issues)でバグを報告したり、質問したり、新機能のリクエストをしてください。もしプルリクエストするならば下記のガイドラインに従うようにしてください。
- 合理的で可能な限りテストカバレッジを維持してください
- なぜPRをしたのかとそのPRの内容について明確にしてください。
- 有意義な貢献をした場合はSPDX FileContributorタグをファイルにつけてください。

[SPDX Annotations](https://spdx.dev)はそのライセンス、著作権表記、コントリビューターについて明確するのために、それぞれのファイルに機械可読な形式で記されています。もし、新しいファイルをレポジトリに追加した場合は、下記のようなSPDXヘッダーを付与していることを確かめてください。

```go
// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt
// SPDX-FileContributor: Your name or alias <optional@email.address>

package name
```

ファイルにそれぞれのコントリビューターの`SPDX-FileContributor`が追加されていることを確認してください、他のファイルを参考にしてください。あなたのこのプロジェクトへのコントリビュートは価値があり、高く評価されます！


## Stargazers over time 🥰
[![Stargazers over time](https://starchart.cc/mochi-mqtt/server.svg)](https://starchart.cc/mochi-mqtt/server)
Mochi MQTTをプロジェクトで使用していますか？ [是非私達に教えてください!](https://github.com/mochi-mqtt/server/issues)

