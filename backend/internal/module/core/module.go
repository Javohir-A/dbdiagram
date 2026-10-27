package core

// Module holds every service of the accounts domain.
type Module struct {
	Auth  *AuthService
	User  *UserService
	Token *TokenService
}

func New(deps Deps) *Module {
	return &Module{
		Auth:  &AuthService{deps: deps},
		User:  &UserService{deps: deps},
		Token: &TokenService{deps: deps},
	}
}
