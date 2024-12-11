package fw

import (
	"github.com/linxlib/inject"
)

// ConfigMapper is the interface for config mapper
// which can make a struct able to load configuration
type ConfigMapper interface {
	LoadWithKey(key string, config any) error
}

// ServiceMapper is the interface for service mapper
// do some initializations for service and then inject something into the container
type ServiceMapper interface {
	// Init do some initializations for service
	// system will Map the result into inject container when no error
	// will panic when error
	Init(config ConfigMapper) (any, error)
}

// IController is the interface for controller
// a controller which implements this interface will have an Init method to provide something (e.g. database orm instance)
type IController interface {
	// Init will be called after the controller is created and the service will be passed to the controller
	// via the provider. The controller should use the provider to get the service instance,
	Init(provider IProvider)
}

// IControllerConfig is the interface for controller
// a controller which implements this interface will have an InitConfig method to load configuration
type IControllerConfig interface {
	InitConfig(config ConfigMapper)
}

// IService is the interface for service
// a service which implements this interface will have an Init method to provide something (e.g. database orm instance)
type IService interface {

	// Init will be called after the service is created and the provider and config will be passed to the service
	// via the provider. The service should use the provider to get the service instance,
	Init(provider IProvider)
}

type IProvider interface {
	inject.Provider
}

// IServiceConfig is the interface for service
// a service which implements this interface will have an InitConfig method to load configuration
type IServiceConfig interface {
	InitConfig(config ConfigMapper)
}
