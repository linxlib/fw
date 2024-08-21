package fw

import (
	"github.com/linxlib/config"
	"github.com/linxlib/inject"
)

type ServiceMapper interface {
	// Init do some initializations for service
	// system will Map the result into inject container when no error
	// will panic when error
	Init(config *config.Config) (any, error)
}

type IController interface {
	// Init will be called after the controller is created and the service will be passed to the controller
	// via the provider. The controller should use the provider to get the service instance,
	Init(provider inject.Provider)
}
