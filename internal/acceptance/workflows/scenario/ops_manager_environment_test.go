package scenario

import (
	"context"
	"testing"
)

func Test_fetchAssociatedStemcellVersion(t *testing.T) {
	_, err := loadEnvironmentVariable("OM_TARGET", "OM_TARGET not set")
	if err != nil {
		t.Skip("OM_TARGET not set")
	}
	ctx := configureStandardFileDescriptors(context.Background())
	v, err := fetchAssociatedStemcellVersion(ctx, "hello")
	if err != nil {
		t.Error(err)
	}
	if v == "" {
		t.Errorf("expected a version")
	}
}
