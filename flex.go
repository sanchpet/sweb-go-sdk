package sweb

import (
	"bytes"
	"fmt"
	"strconv"
)

// FlexInt is an int64 that decodes from EITHER a JSON number (4) or a quoted
// string ("4"), and from null (→ 0). The SpaceWeb API returns many numeric
// fields inconsistently as one or the other across nodes/plans, so a plain int
// crashes on the string form (the plan_price/ram class of bug). Marshals back
// as a bare JSON number.
type FlexInt int64

// UnmarshalJSON accepts 4, "4", "" and null.
func (f *FlexInt) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	s := string(bytes.Trim(b, `"`))
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("sweb: decode FlexInt from %s: %w", b, err)
	}
	*f = FlexInt(n)
	return nil
}

// MarshalJSON emits a bare JSON number.
func (f FlexInt) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(f), 10)), nil
}

// FlexFloat is FlexInt's fractional sibling for money fields: decodes from a
// JSON number (0.9), a quoted string ("0.9"), or null (→ 0). Marshals back as a
// bare JSON number.
type FlexFloat float64

// UnmarshalJSON accepts 0.9, "0.9", "" and null.
func (f *FlexFloat) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	s := string(bytes.Trim(b, `"`))
	if s == "" {
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("sweb: decode FlexFloat from %s: %w", b, err)
	}
	*f = FlexFloat(n)
	return nil
}

// MarshalJSON emits a bare JSON number (shortest round-trippable form).
func (f FlexFloat) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatFloat(float64(f), 'f', -1, 64)), nil
}
