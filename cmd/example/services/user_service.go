package services

import (
	"fmt"

	"github.com/linxlib/fw/v2/cmd/example/models"
)

// @Service
type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) BuildModifyUserResponse(query models.UserQuery, userID int, role string, db *DemoDB) map[string]any {
	dbName := ""
	if db != nil {
		dbName = db.Name
	}
	return map[string]any{
		"name":    query.Name,
		"age":     query.Age,
		"user_id": userID,
		"role":    role,
		"db":      dbName,
	}
}

func (s *UserService) BuildWSReply(msg []byte) []byte {
	return []byte(fmt.Sprintf("echo: %s", string(msg)))
}
