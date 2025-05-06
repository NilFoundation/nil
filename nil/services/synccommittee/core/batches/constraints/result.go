package constraints

import (
	"fmt"
)

//go:generate stringer -type=CheckResultType -trimprefix=CheckResultType

type CheckResultType uint8

const (
	_ CheckResultType = iota

	// CheckResultTypeCanBeExtended indicates that the batch can be further extended with additional blocks.
	CheckResultTypeCanBeExtended

	// CheckResultTypeShouldBeSealed indicates that the batch should be finalized and cannot be further extended.
	CheckResultTypeShouldBeSealed

	// CheckResultTypeShouldBeDiscarded indicates that the batch is not valid for processing and should be discarded.
	CheckResultTypeShouldBeDiscarded
)

var severity = map[CheckResultType]uint8{
	CheckResultTypeCanBeExtended:     1,
	CheckResultTypeShouldBeSealed:    2,
	CheckResultTypeShouldBeDiscarded: 3,
}

type CheckResult struct {
	Type    CheckResultType
	Details string
}

func (r *CheckResult) String() string {
	return fmt.Sprintf("%s: %s", r.Type, r.Details)
}

func (r *CheckResult) JoinWith(other *CheckResult) {
	if severity[other.Type] > severity[r.Type] {
		r.Type = other.Type
	}

	var joinedDetails string
	switch {
	case r.Details == "":
		joinedDetails = other.Details
	case other.Details == "":
		joinedDetails = r.Details
	default:
		joinedDetails = r.Details + "; " + other.Details
	}

	r.Details = joinedDetails
}

func (r *CheckResult) CanBeExtended() bool {
	return r.Type == CheckResultTypeCanBeExtended
}

func newCheckResult(resultType CheckResultType, format string, args ...any) CheckResult {
	return CheckResult{
		Type:    resultType,
		Details: fmt.Sprintf(format, args...),
	}
}

func canBeExtended() *CheckResult {
	result := newCheckResult(CheckResultTypeCanBeExtended, "")
	return &result
}

func shouldBeSealed(format string, args ...any) *CheckResult {
	result := newCheckResult(CheckResultTypeShouldBeSealed, format, args...)
	return &result
}

func shouldBeDiscarded(format string, args ...any) *CheckResult {
	result := newCheckResult(CheckResultTypeShouldBeDiscarded, format, args...)
	return &result
}
