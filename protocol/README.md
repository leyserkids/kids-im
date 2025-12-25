> ## 说明
>
> Fork 自 [openimsdk/protocol](https://github.com/openimsdk/protocol)
>
> 基于版本：`v0.0.72-alpha.78`
>
> ### 修改记录
> - 支持群已读，并统一单聊已读和群已读的实现方式

---

# protocol

```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

More to read [regenerate-grpc-code](https://grpc.io/docs/languages/go/quickstart/#regenerate-grpc-code).


 - Run `mage` to gen rpc_caller.