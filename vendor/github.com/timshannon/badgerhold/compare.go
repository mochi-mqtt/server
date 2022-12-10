// Copyright 2019 Tim Shannon. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package badgerhold

import (
	"fmt"
	"math/big"
	"reflect"
	"time"
)

// ErrTypeMismatch is the error thrown when two types cannot be compared
type ErrTypeMismatch struct {
	Value interface{}
	Other interface{}
}

func (e *ErrTypeMismatch) Error() string {
	return fmt.Sprintf("%v (%T) cannot be compared with %v (%T)", e.Value, e.Value, e.Other, e.Other)
}

//Comparer compares a type against the encoded value in the store. The result should be 0 if current==other,
// -1 if current < other, and +1 if current > other.
// If a field in a struct doesn't specify a comparer, then the default comparison is used (convert to string and compare)
// this interface is already handled for standard Go Types as well as more complex ones such as those in time and big
// an error is returned if the type cannot be compared
// The concrete type will always be passedin, not a pointer
type Comparer interface {
	Compare(other interface{}) (int, error)
}

func (c *Criterion) compare(rowValue, criterionValue interface{}, currentRow interface{}) (int, error) {
	if rowValue == nil || criterionValue == nil {
		if rowValue == criterionValue {
			return 0, nil
		}
		return 0, &ErrTypeMismatch{rowValue, criterionValue}
	}

	if _, ok := criterionValue.(Field); ok {
		fVal := reflect.ValueOf(currentRow).Elem().FieldByName(string(criterionValue.(Field)))
		if !fVal.IsValid() {
			return 0, fmt.Errorf("The field %s does not exist in the type %s", criterionValue,
				reflect.TypeOf(currentRow))
		}

		criterionValue = fVal.Interface()
	}

	value := rowValue

	for reflect.TypeOf(value).Kind() == reflect.Ptr {
		value = reflect.ValueOf(value).Elem().Interface()
	}

	other := criterionValue
	for reflect.TypeOf(other).Kind() == reflect.Ptr {
		other = reflect.ValueOf(other).Elem().Interface()
	}

	return compare(value, other)
}

func compare(value, other interface{}) (int, error) {
	switch t := value.(type) {
	case time.Time:
		tother, ok := other.(time.Time)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(time.Time).Equal(tother) {
			return 0, nil
		}

		if value.(time.Time).Before(tother) {
			return -1, nil
		}
		return 1, nil
	case big.Float:
		o, ok := other.(big.Float)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		v := value.(big.Float)

		return v.Cmp(&o), nil
	case big.Int:
		o, ok := other.(big.Int)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		v := value.(big.Int)

		return v.Cmp(&o), nil
	case big.Rat:
		o, ok := other.(big.Rat)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		v := value.(big.Rat)

		return v.Cmp(&o), nil
	case int:
		tother, ok := other.(int)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(int) == tother {
			return 0, nil
		}

		if value.(int) < tother {
			return -1, nil
		}
		return 1, nil
	case int8:
		tother, ok := other.(int8)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(int8) == tother {
			return 0, nil
		}

		if value.(int8) < tother {
			return -1, nil
		}
		return 1, nil

	case int16:
		tother, ok := other.(int16)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(int16) == tother {
			return 0, nil
		}

		if value.(int16) < tother {
			return -1, nil
		}
		return 1, nil
	case int32:
		tother, ok := other.(int32)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(int32) == tother {
			return 0, nil
		}

		if value.(int32) < tother {
			return -1, nil
		}
		return 1, nil

	case int64:
		tother, ok := other.(int64)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(int64) == tother {
			return 0, nil
		}

		if value.(int64) < tother {
			return -1, nil
		}
		return 1, nil
	case uint:
		tother, ok := other.(uint)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(uint) == tother {
			return 0, nil
		}

		if value.(uint) < tother {
			return -1, nil
		}
		return 1, nil
	case uint8:
		tother, ok := other.(uint8)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(uint8) == tother {
			return 0, nil
		}

		if value.(uint8) < tother {
			return -1, nil
		}
		return 1, nil

	case uint16:
		tother, ok := other.(uint16)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(uint16) == tother {
			return 0, nil
		}

		if value.(uint16) < tother {
			return -1, nil
		}
		return 1, nil
	case uint32:
		tother, ok := other.(uint32)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(uint32) == tother {
			return 0, nil
		}

		if value.(uint32) < tother {
			return -1, nil
		}
		return 1, nil

	case uint64:
		tother, ok := other.(uint64)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(uint64) == tother {
			return 0, nil
		}

		if value.(uint64) < tother {
			return -1, nil
		}
		return 1, nil
	case float32:
		tother, ok := other.(float32)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(float32) == tother {
			return 0, nil
		}

		if value.(float32) < tother {
			return -1, nil
		}
		return 1, nil
	case float64:
		tother, ok := other.(float64)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(float64) == tother {
			return 0, nil
		}

		if value.(float64) < tother {
			return -1, nil
		}
		return 1, nil
	case string:
		tother, ok := other.(string)
		if !ok {
			return 0, &ErrTypeMismatch{t, other}
		}

		if value.(string) == tother {
			return 0, nil
		}

		if value.(string) < tother {
			return -1, nil
		}
		return 1, nil
	case Comparer:
		return value.(Comparer).Compare(other)
	default:
		valS := fmt.Sprintf("%s", value)
		otherS := fmt.Sprintf("%s", other)
		if valS == otherS {
			return 0, nil
		}

		if valS < otherS {
			return -1, nil
		}

		return 1, nil
	}

}
