package duration

import (
	"bytes"
	"fmt"
	"time"
)

type Duration time.Duration

func (r *Duration) MarshalJSON() ([]byte, error) {
	bs := []byte{'"'}
	bs = append(bs)
	bs = append(bs, time.Duration(*r).String()...)
	bs = append(bs, '"')
	return bs, nil
}

func (r *Duration) UnmarshalJSON(bs []byte) error {
	bs = bytes.Trim(bs, `"`)
	dur, err := time.ParseDuration(string(bs))
	if err != nil {
		return fmt.Errorf("invalid duration string:%w", err)
	}
	*r = Duration(dur)
	return nil
}
