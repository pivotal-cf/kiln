package scenario

import (
	"context"
	"testing"
)

// Test_fetchAssociatedStemcellVersion will fail if you do not have a tile uploaded to the target ops manager
// unset OM_TARGET when you want this test not to run.
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
