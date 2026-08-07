package marketplace

import (
	"context"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

type Type int

const (
	TypePlugin Type = iota
	TypeSkill
	TypePersona
	TypeTheme
)

func (t Type) String() string {
	switch t {
	case TypePlugin:
		return "plugin"
	case TypeSkill:
		return "skill"
	case TypePersona:
		return "persona"
	case TypeTheme:
		return "theme"
	default:
		return strconst.StrUnknown
	}
}

type Listing struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Version      string    `json:"version"`
	Author       string    `json:"author"`
	Type         Type      `json:"type"`
	Downloads    int       `json:"downloads"`
	Rating       float64   `json:"rating"`
	Tags         []string  `json:"tags"`
	SourceURL    string    `json:"source_url"`
	InstallCount int       `json:"install_count"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Filter struct {
	Type      *Type
	MinRating *float64
	Tags      []string
	SortBy    string
	SortOrder string
}

type Store interface {
	List(ctx context.Context, filter Filter) ([]Listing, error)
	Search(ctx context.Context, query string) ([]Listing, error)
	Get(ctx context.Context, id string) (*Listing, error)
	Register(ctx context.Context, listing Listing) error
	Delete(ctx context.Context, id string) error
}
