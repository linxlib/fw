# fw

framework 简称

继 github.com/linxlib/kapi 之后，重写的版本，将对gin的依赖改为fasthttp，
本来想直接使用net/http的，但是想到可能需要重新实现那些请求解析、路由什么的，
最终还是选择了fasthttp

希望成为一个代码还没怎么敲，诶~，写完编译了这样的框架

## 特性

- 支持 @Attribute 形式的注释来注入中间件和注册路由
- 支持参数的映射和注入, 减少全局变量的使用，不用写 Unmarshal(xxx) 和 Bind(xxx)
- 方便编写和使用的中间件，写好Use一下，在需要的地方 @ 一下
- 支持结构体的embedded写法

更多特性请查看 github.com/linxlib/fw_example

## 未来增加

- 基于本框架完成一个admin管理后台
- benchmark
- ...


