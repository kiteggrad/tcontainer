package tcontainer

import (
	"context"
	"log"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	err := MustNewPool("").Prune(context.Background())
	if err != nil {
		log.Fatal("failed to Prune:", err)
	}

	goleak.VerifyTestMain(m)
}
