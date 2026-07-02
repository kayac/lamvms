package lamvms

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
)

func versionNewerFirst(a, b types.MicrovmImageVersionSummary) int {
	ta := createdAtOrZero(a.CreatedAt)
	tb := createdAtOrZero(b.CreatedAt)
	switch {
	case ta.After(tb):
		return -1
	case ta.Before(tb):
		return 1
	default:
		return 0
	}
}

func createdAtOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
