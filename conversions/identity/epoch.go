package identity

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wingitman/atob/conversions"
)

func init() {
	conversions.Register(epochToHuman{})
	conversions.Register(humanToEpoch{})
	conversions.Register(epochNow{})
}

type epochToHuman struct{}

func (epochToHuman) Name() string        { return "epoch-human" }
func (epochToHuman) Category() string    { return "identity" }
func (epochToHuman) Description() string { return "Convert Unix epoch (seconds) to human-readable UTC datetime" }

func (epochToHuman) Convert(input string) (string, error) {
	secs, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid epoch (expected integer seconds): %w", err)
	}
	t := time.Unix(secs, 0).UTC()
	return t.Format(time.RFC3339), nil
}

type humanToEpoch struct{}

func (humanToEpoch) Name() string        { return "human-epoch" }
func (humanToEpoch) Category() string    { return "identity" }
func (humanToEpoch) Description() string { return "Convert RFC3339 / common datetime string to Unix epoch" }

var timeFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"01/02/2006",
	"02-01-2006",
}

func (humanToEpoch) Convert(input string) (string, error) {
	s := strings.TrimSpace(input)
	for _, layout := range timeFormats {
		t, err := time.Parse(layout, s)
		if err == nil {
			return strconv.FormatInt(t.Unix(), 10), nil
		}
	}
	return "", fmt.Errorf("unrecognised datetime format %q — try RFC3339 (e.g. 2006-01-02T15:04:05Z)", s)
}

type epochNow struct{}

func (epochNow) Name() string        { return "epoch-now" }
func (epochNow) Category() string    { return "identity" }
func (epochNow) Description() string { return "Print the current Unix epoch (ignores input)" }

func (epochNow) Convert(_ string) (string, error) {
	return strconv.FormatInt(time.Now().Unix(), 10), nil
}
