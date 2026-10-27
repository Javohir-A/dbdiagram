package model

// Role is what a caller may do with a folder or a diagram. The empty Role means
// "no access at all": the resource then does not exist as far as they know.
type Role string

const (
	RoleNone   Role = ""
	RoleViewer Role = "viewer"
	RoleEditor Role = "editor"
	RoleOwner  Role = "owner"
)

// Rank orders the roles: viewer < editor < owner; no access is 0.
func (r Role) Rank() int {
	switch r {
	case RoleViewer:
		return 1
	case RoleEditor:
		return 2
	case RoleOwner:
		return 3
	default:
		return 0
	}
}

// AtLeast reports whether r grants everything min does.
func (r Role) AtLeast(min Role) bool { return r.Rank() >= min.Rank() && r.Rank() > 0 }

// Shareable are the roles a member can be given; ownership is never granted.
func (r Role) Shareable() bool { return r == RoleEditor || r == RoleViewer }

// User is an account as every layer sees it. The password hash is not part of
// it: it never leaves the storage layer except through Credentials.
type User struct {
	Guid      string `json:"guid"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// AuthResult is what register, login and refresh answer with.
type AuthResult struct {
	User         *User  `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds until the access token expires
}

// Member is a person with access to one folder or diagram. The owner is listed
// too, with RoleOwner.
type Member struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      Role   `json:"role"`
	CreatedAt string `json:"created_at"`
}

// Revision is the cheap "has it changed?" answer the editor polls.
type Revision struct {
	Revision      int64  `json:"revision"`
	UpdatedAt     string `json:"updated_at"`
	UpdatedByName string `json:"updated_by_name"`
}

// TokenScope limits what a personal access token may do.
type TokenScope string

const (
	ScopeRead  TokenScope = "read"  // safe requests and read-only tools
	ScopeWrite TokenScope = "write" // everything a user can do except account management
)

func (s TokenScope) Valid() bool { return s == ScopeRead || s == ScopeWrite }

// AccessToken is a personal access token as listed. The secret is not part of
// it: it exists only in CreatedAccessToken, once.
type AccessToken struct {
	Guid       string     `json:"guid"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scope      TokenScope `json:"scope"`
	ExpiresAt  string     `json:"expires_at"`
	LastUsedAt string     `json:"last_used_at"`
	CreatedAt  string     `json:"created_at"`
}

// CreatedAccessToken is the one response that carries the secret.
type CreatedAccessToken struct {
	AccessToken
	Token string `json:"token"`
}
