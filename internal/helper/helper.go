package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math/big"
	"strconv"
	"time"
)

func ConvertTimeToProtoTimestamp[T time.Time | *time.Time](t T) *timestamppb.Timestamp {
	switch v := any(t).(type) {
	case time.Time:
		if !v.IsZero() {
			return timestamppb.New(v)
		}
	case *time.Time:
		if v != nil && !v.IsZero() {
			return timestamppb.New(*v)
		}
	}
	return nil
}

// CompareDecimal128 compares two primitive.Decimal128 values.
// It returns:
// -1 if d1 < d2
// 0 if d1 == d2
// 1 if d1 > d2
// An error if conversion to BigFloat fails.
func CompareDecimal128(d1, d2 primitive.Decimal128) (int, error) {
	f1, _, err := new(big.Float).Parse(d1.String(), 10)
	if err != nil {
		return 0, fmt.Errorf("failed to convert d1 to big.Float: %w", err)
	}
	f2, _, err := new(big.Float).Parse(d2.String(), 10)
	if err != nil {
		return 0, fmt.Errorf("failed to convert d2 to big.Float: %w", err)
	}
	return f1.Cmp(f2), nil
}

// SubDecimal128 subtracts d2 from d1 (d1 - d2).
// It returns the result as a primitive.Decimal128.
// An error is returned if conversion to BigFloat or back to Decimal128 fails.
func SubDecimal128(d1, d2 primitive.Decimal128) (primitive.Decimal128, error) {
	f1, _, err := new(big.Float).SetPrec(big.MaxPrec).Parse(d1.String(), 10)
	if err != nil {
		return primitive.Decimal128{}, fmt.Errorf("failed to convert d1 to big.Float: %w", err)
	}
	f2, _, err := new(big.Float).SetPrec(big.MaxPrec).Parse(d2.String(), 10)
	if err != nil {
		return primitive.Decimal128{}, fmt.Errorf("failed to convert d2 to big.Float: %w", err)
	}

	resultFloat := new(big.Float).Sub(f1, f2)

	resultDecimal, err := primitive.ParseDecimal128(resultFloat.String())
	if err != nil {
		return primitive.Decimal128{}, fmt.Errorf("failed to convert result to Decimal128: %w", err)
	}

	return resultDecimal, nil
}

// AddDecimal128 adds two primitive.Decimal128 values (d1 + d2).
// It returns the result as a primitive.Decimal128.
func AddDecimal128(d1, d2 primitive.Decimal128) (primitive.Decimal128, error) {
	f1, _, err := new(big.Float).SetPrec(big.MaxPrec).Parse(d1.String(), 10)
	if err != nil {
		return primitive.Decimal128{}, fmt.Errorf("failed to convert d1 to big.Float: %w", err)
	}
	f2, _, err := new(big.Float).SetPrec(big.MaxPrec).Parse(d2.String(), 10)
	if err != nil {
		return primitive.Decimal128{}, fmt.Errorf("failed to convert d2 to big.Float: %w", err)
	}

	resultFloat := new(big.Float).Add(f1, f2)

	resultDecimal, err := primitive.ParseDecimal128(resultFloat.String())
	if err != nil {
		return primitive.Decimal128{}, fmt.Errorf("failed to convert result to Decimal128: %w", err)
	}

	return resultDecimal, nil
}

func ConvertStringsToObjectID(s []string) ([]primitive.ObjectID, error) {
	if len(s) == 0 {
		return nil, errors.New("empty slice")
	}

	ss := make([]primitive.ObjectID, 0, len(s))

	for _, v := range s {
		oid, err := primitive.ObjectIDFromHex(v)
		if err != nil {
			return nil, errors.New("include invalid string")
		}
		ss = append(ss, oid)
	}

	return ss, nil
}

func DebugPrintQuery(v interface{}) {
	pipelineJSON, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println("Final query:")
	fmt.Println(string(pipelineJSON))
}

// Decimal128ToInt64 converts a primitive.Decimal128 to an int64.
// It truncates any fractional part.
// It returns an error if the value is a special value (NaN, Infinity) or is out of the int64 range.
func Decimal128ToInt64(d primitive.Decimal128) (int64, error) {
	// Check for special values first.
	if d.IsNaN() || d.IsInf() != 0 {
		return 0, fmt.Errorf("cannot convert special Decimal128 value (NaN or Infinity) to int64")
	}
	d.IsInf()

	// Parse the string representation into a big.Float.
	f, _, err := new(big.Float).SetPrec(big.MaxPrec).Parse(d.String(), 10)
	if err != nil {
		return 0, fmt.Errorf("failed to parse Decimal128 string '%s' into big.Float: %w", d.String(), err)
	}

	// Convert the big.Float to a big.Int, which truncates the fractional part.
	i, _ := f.Int(nil)
	if i == nil {
		return 0, fmt.Errorf("failed to convert big.Float to big.Int for value %s", f.String())
	}

	// Check if the resulting big.Int fits into an int64.
	if !i.IsInt64() {
		return 0, fmt.Errorf("value %s is out of int64 range", i.String())
	}

	// Return the int64 value.
	return i.Int64(), nil
}

// NegateDecimal128 multiplies a Decimal128 value by -1.
func NegateDecimal128(d primitive.Decimal128) (primitive.Decimal128, error) {
	// Convert the decimal to a big.Float
	f, _, err := new(big.Float).SetPrec(big.MaxPrec).Parse(d.String(), 10)
	if err != nil {
		return primitive.Decimal128{}, fmt.Errorf("failed to convert d to big.Float: %w", err)
	}

	// Create a big.Float for -1
	minusOne := big.NewFloat(-1)

	// Multiply
	resultFloat := new(big.Float).Mul(f, minusOne)

	// Convert the result back to Decimal128
	resultDecimal, err := primitive.ParseDecimal128(resultFloat.String())
	if err != nil {
		return primitive.Decimal128{}, fmt.Errorf("failed to convert result to Decimal128: %w", err)
	}

	return resultDecimal, nil
}

// Decimal128ToFloat64 converts a primitive.Decimal128 to a float64.
// It returns an error if the underlying string conversion fails.
func Decimal128ToFloat64(d primitive.Decimal128) (float64, error) {
	return strconv.ParseFloat(d.String(), 64)
}
