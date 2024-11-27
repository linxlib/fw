package fw

import (
	"github.com/linxlib/inject"
)

type ConfigMapper interface {
	LoadWithKey(key string, config any) error
}

type ServiceMapper interface {
	// Init do some initializations for service
	// system will Map the result into inject container when no error
	// will panic when error
	Init(config ConfigMapper) (any, error)
}

type IController interface {
	// Init will be called after the controller is created and the service will be passed to the controller
	// via the provider. The controller should use the provider to get the service instance,
	Init(provider inject.Provider)
}

type IControllerConfig interface {
	InitConfig(config ConfigMapper)
}

type IService interface {

	// Init will be called after the service is created and the provider and config will be passed to the service
	// via the provider. The service should use the provider to get the service instance,
	Init(provider inject.Provider)
}
type IServiceConfig interface {
	InitConfig(config ConfigMapper)
}
