package wspooler

import (
	"context"
	"testing"

	"github.com/kychandar/ottam/services/valkey"
)

const addr = "100.130.101.125:30079"

func TestWSPooler(t *testing.T) {
	conn, _ := valkey.New(addr)
	pooler := New(context.TODO(), conn)
	pooler.Start()

}
