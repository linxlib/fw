package services

type StoreService struct {
}

func (s *StoreService) Name() string {
	return "StoreService"
}

func (s *StoreService) Cmd() string {
	return "Service"
}

func (s *StoreService) Index() int {
	return 0
}
