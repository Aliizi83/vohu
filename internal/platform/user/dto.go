package user

type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"omitempty,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type UpdateUserRequest struct {
	Email   string `json:"email" binding:"omitempty,email"`
	Enabled *bool  `json:"enabled"`
}

type Response struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Enabled  bool   `json:"enabled"`
}

func toResponse(u User) Response {
	return Response{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
		Enabled:  u.Enabled,
	}
}
