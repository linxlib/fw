package services

type IService interface {
	Name() string
	Cmd() string
	Index() int
}
