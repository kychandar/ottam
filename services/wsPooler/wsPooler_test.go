package wspooler

import (
	"testing"
	"time"

	"github.com/kychandar/ottam/services/valkey"
)

const addr = "100.130.101.125:30079"

func TestWSPooler(t *testing.T) {
	conn := valkey.New(addr)
	pooler := New(conn)
	pooler.Start()

	time.Sleep(100 * time.Second)
}
