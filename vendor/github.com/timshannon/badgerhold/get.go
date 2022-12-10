// Copyright 2019 Tim Shannon. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package badgerhold

import (
	"errors"
	"reflect"
	"strings"

	"github.com/dgraph-io/badger"
)

// ErrNotFound is returned when no data is found for the given key
var ErrNotFound = errors.New("No data found for this key")

// Get retrieves a value from badgerhold and puts it into result.  Result must be a pointer
func (s *Store) Get(key, result interface{}) error {
	return s.Badger().View(func(tx *badger.Txn) error {
		return s.TxGet(tx, key, result)
	})
}

// TxGet allows you to pass in your own badger transaction to retrieve a value from the badgerhold and puts it
// into result
func (s *Store) TxGet(tx *badger.Txn, key, result interface{}) error {
	storer := newStorer(result)

	gk, err := encodeKey(key, storer.Type())

	if err != nil {
		return err
	}

	item, err := tx.Get(gk)
	if err == badger.ErrKeyNotFound {
		return ErrNotFound
	}

	err = item.Value(func(value []byte) error {
		return decode(value, result)
	})

	if err != nil {
		return err
	}

	tp := reflect.TypeOf(result)
	for tp.Kind() == reflect.Ptr {
		tp = tp.Elem()
	}

	var keyField string

	for i := 0; i < tp.NumField(); i++ {
		if strings.Contains(string(tp.Field(i).Tag), BadgerholdKeyTag) {
			keyField = tp.Field(i).Name
			break
		}
	}

	if keyField != "" {
		err := decodeKey(gk, reflect.ValueOf(result).Elem().FieldByName(keyField).Addr().Interface(), storer.Type())
		if err != nil {
			return err
		}
	}

	return nil
}

// Find retrieves a set of values from the badgerhold that matches the passed in query
// result must be a pointer to a slice.
// The result of the query will be appended to the passed in result slice, rather than the passed in slice being
// emptied.
func (s *Store) Find(result interface{}, query *Query) error {
	return s.Badger().View(func(tx *badger.Txn) error {
		return s.TxFind(tx, result, query)
	})
}

// TxFind allows you to pass in your own badger transaction to retrieve a set of values from the badgerhold
func (s *Store) TxFind(tx *badger.Txn, result interface{}, query *Query) error {
	return findQuery(tx, result, query)
}
