package testdata

type User struct {
	Name string `json:"name" db:"user_name"`
	Age  int    `json:"age"`
}

func (u *User) GetName() string {
	return u.Name
}

func (u *User) Copy() *User {
	return &User{Name: u.Name, Age: u.Age}
}

func CreateUser(name string, age int) *User {
	return &User{Name: name, Age: age}
}

type Reader interface {
	Read() error
}

var DefaultUser = &User{Name: "default"}

const Version = "1.0.0"
