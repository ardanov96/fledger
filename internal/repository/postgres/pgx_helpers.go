package postgres

import (
	"time"

	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// nanoToTime converts a unix nanosecond timestamp to time.Time.
// Used to translate from domain's int64 filter to SQL's timestamptz.
func nanoToTime(nano int64) time.Time {
	return time.Unix(0, nano)
}

// timeToNano converts time.Time to unix nanoseconds.
func timeToNano(t time.Time) int64 {
	return t.UnixNano()
}

// amountFromMinor wraps money.NewFromMinor for convenient DTO→domain conversion.
func amountFromMinor(minor int64) money.Money {
	return money.NewFromMinor(minor)
}
