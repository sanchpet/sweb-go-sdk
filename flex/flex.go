// Package flex holds the tolerant JSON scalar types the SpaceWeb API forces on
// every consumer: the API quotes numeric fields inconsistently (bare 1, quoted
// "1024", or null) and even returns money as int-or-float, so a plain int/float
// panics on real payloads. flex.Int and flex.Float decode either form. It is a
// public leaf: it imports nothing from this module.
package flex

import (
	"bytes"
	"fmt"
	"strconv"
)

// Int is an int64 that decodes from EITHER a JSON number (4) or a quoted string
// ("4"), and from null (→ 0). The SpaceWeb API returns many numeric fields
// inconsistently as one or the other across nodes/plans, so a plain int crashes
// on the string form (the plan_price/ram class of bug). Marshals back as a bare
// JSON number.
type Int int64

// UnmarshalJSON accepts 4, "4", "" and null.
func (f *Int) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	s := string(bytes.Trim(b, `"`))
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("sweb: decode flex.Int from %s: %w", b, err)
	}
	*f = Int(n)
	return nil
}

// MarshalJSON emits a bare JSON number.
func (f Int) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(f), 10)), nil
}

// Float is Int's fractional sibling for money fields: decodes from a JSON number
// (0.9), a quoted string ("0.9"), or null (→ 0). Marshals back as a bare JSON
// number.
type Float float64

// UnmarshalJSON accepts 0.9, "0.9", "" and null.
func (f *Float) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	s := string(bytes.Trim(b, `"`))
	if s == "" {
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("sweb: decode flex.Float from %s: %w", b, err)
	}
	*f = Float(n)
	return nil
}

// MarshalJSON emits a bare JSON number (shortest round-trippable form).
func (f Float) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatFloat(float64(f), 'f', -1, 64)), nil
}
