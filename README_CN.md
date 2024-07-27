# fw

framework 简称

继 github.com/linxlib/kapi 之后，重写的版本，将对gin的依赖改为fasthttp，
本来想直接使用net/http的，但是想到可能需要重新实现那些请求解析、路由什么的，
最终还是选择了fasthttp


## 开发过程中遇到的问题

- `json-iterator/go` 在反序列化 `map[string]*File` 类似这样的结构时，
会丢失数据，比如我遇到的就会丢失一个key。随后又试了 `goccy/go-json`，也是类似的情况，
不知道是不是要特殊配置。还是先用自带的




## TODO List

- [x] WebsocketHub 作为 控制器级别中间件
- [ ] RecoveryMiddleware 作为 全局中间件
- [ ] 日志类
- [x] 配置写出
- [ ] Swagger
- [ ] pprof middleware
- [ ] WebLogMiddleware
- [x] ServerDownMiddleware 作为全局中间件
- [ ] ResponseRewriter 作为全局或ctl中间件
- [ ] CorsMiddleware 全局中间件
- [ ] Crud 中间件
- [ ] 数据库或者服务的注入
- [ ] 运行为 系统服务 
- [ ] 开发模式
- [ ] 检测代码自动重启功能
- [ ] Grpc
- [ ] 泛型支持
- [ ] AuthMiddleware 全局或ctl或method