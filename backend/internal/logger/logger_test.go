package logger

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeAttributeUsesUTC(t *testing.T) {
	local := time.FixedZone("CEST", 2*60*60)
	value := time.Date(2026, time.July, 31, 14, 25, 57, 0, local)

	got := utcTimeAttr(nil, slog.Time(slog.TimeKey, value))
	require.Equal(t, time.UTC, got.Value.Time().Location())
	require.Equal(t, "2026-07-31T12:25:57Z", got.Value.Time().Format(time.RFC3339))
}
