package middlewares

import (
	"github.com/linxlib/fw"
	"github.com/redis/go-redis/v9"
)

// RedisMiddleware used for injecting Redis service into method (with *RedisService param)
type RedisMiddleware struct {
	fw.MiddlewareGlobal
	option *redis.Options
	client *redis.Client
}

func (r *RedisMiddleware) CloneAsMethod() fw.IMiddlewareMethod {
	return r.CloneAsCtl()
}

func (r *RedisMiddleware) HandlerMethod(h fw.HandlerFunc) fw.HandlerFunc {
	// connect redis
	r.client = redis.NewClient(r.option)

	return func(context *fw.Context) {
		context.Map(r.client)
		h(context)
	}

}

func (r *RedisMiddleware) CloneAsCtl() fw.IMiddlewareCtl {
	return NewRedisMiddleware(r.option)
}

func (r *RedisMiddleware) HandlerController(string) *fw.RouteItem {
	return &fw.RouteItem{
		Method:     "",
		Path:       "",
		IsHide:     false,
		H:          nil,
		Middleware: r,
	}
}

// NewRedisMiddleware returns RedisMiddleware with redis.Options
// refer to: https://redis.uptrace.dev/guide/go-redis.html#connecting-to-redis-server
func NewRedisMiddleware(opt *redis.Options) *RedisMiddleware {
	return &RedisMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal("RedisMiddleware"),
		option:           opt,
	}
}

// NewRedisMiddlewareWithUrl returns RedisMiddleware with redis url
// refer to: https://redis.uptrace.dev/guide/go-redis.html#connecting-to-redis-server
func NewRedisMiddlewareWithUrl(url string) *RedisMiddleware {
	opt, err := redis.ParseURL(url)
	if err != nil {
		panic(err)
	}
	return &RedisMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal("RedisMiddleware"),
		option:           opt,
	}
}
