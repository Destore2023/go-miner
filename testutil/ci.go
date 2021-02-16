package testutil

import (
	"os"
	"testing"
)

const envUseCI = "SKT_CI"

func SkipCI(t *testing.T) {
	if os.Getenv(envUseCI) == "" {
		t.Skip("Skip SKT CI")
	}
}
