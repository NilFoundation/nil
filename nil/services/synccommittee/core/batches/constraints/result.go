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

func (r *CheckResult) Join(other *CheckResult) *CheckResult {
	var joinedType CheckResultType
	if severity[r.Type] >= severity[other.Type] {
		joinedType = r.Type
	} else {
		joinedType = other.Type
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

	return &CheckResult{
		Type:    joinedType,
		Details: joinedDetails,
	}
}

func newCheckResult(resultType CheckResultType, format string, args ...interface{}) CheckResult {
	return CheckResult{
		Type:    resultType,
		Details: fmt.Sprintf(format, args...),
	}
}

func canBeExtended() *CheckResult {
	result := newCheckResult(CheckResultTypeCanBeExtended, "")
	return &result
}

func shouldBeSealed(format string, args ...interface{}) *CheckResult {
	result := newCheckResult(CheckResultTypeShouldBeSealed, format, args...)
	return &result
}

func shouldBeDiscarded(format string, args ...interface{}) *CheckResult {
	result := newCheckResult(CheckResultTypeShouldBeDiscarded, format, args...)
	return &result
}
