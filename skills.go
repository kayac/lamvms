package lamvms

import (
	"context"
	"embed"

	"github.com/Songmu/skillsmith"
	"github.com/kayac/lamvms/skillscmd"
)

//go:embed skills
var skillsFS embed.FS

func dispatchSkills(ctx context.Context, opts *skillscmd.Commands) error {
	version := Version
	if version == "" {
		version = "v0.0.0-dev"
	}
	s, err := skillsmith.New("lamvms", version, skillsFS)
	if err != nil {
		return err
	}
	return opts.Run(ctx, s)
}
