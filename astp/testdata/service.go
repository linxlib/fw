package testdata

// @Service
// @Path("/api")
type Service struct {
	Name string `json:"name"`
}

// @GET
// @Path("/users")
func (s *Service) GetUsers() []User {
	return nil
}

// @POST
// @Path("/users")
func (s *Service) CreateUser(u *User) error {
	return nil
}
