package services

// DemoDB is a lightweight stand-in for a singleton database client.
// In real projects this can be replaced with *gorm.DB.
// @Service
type DemoDB struct {
	Name string
}

func NewDemoDB(name string) *DemoDB {
	return &DemoDB{Name: name}
}
