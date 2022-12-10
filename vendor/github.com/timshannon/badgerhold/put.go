// Copyright 2019 Tim Shannon. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package badgerhold

import (
	"errors"
	"reflect"

	"github.com/dgraph-io/badger"
)

// ErrKeyExists is the error returned when data is being Inserted for a Key that already exists
var ErrKeyExists = errors.New("This Key already exists in badgerhold for this type")

// ErrUniqueExists is the error thrown when data is being inserted for a unique constraint value that already exists
var ErrUniqueExists = errors.New("This value cannot be written due to the unique constraint on the field")

// sequence tells badgerhold to insert the key as the next sequence in the bucket
type sequence struct{}

// NextSequence is used to create a sequential key for inserts
// Inserts a uint64 as the key
// store.Insert(badgerhold.NextSequence(), data)
func NextSequence() interface{} {
	return sequence{}
}

// Insert inserts the passed in data into the the badgerhold
//
// If the the key already exists in the badgerhold, then an ErrKeyExists is returned
// If the data struct has a field tagged as `badgerholdKey` and it is the same type
// as the Insert key, AND the data struct is passed by reference, AND the key field
// is currently set to the zero-value for that type, then that field will be set to
// the value of the insert key.
//
// To use this with badgerhold.NextSequence() use a type of `uint64` for the key field.
func (s *Store) Insert(key, data interface{}) error {
	return s.Badger().Update(func(tx *badger.Txn) error {
		return s.TxInsert(tx, key, data)
	})
}

// TxInsert is the same as Insert except it allows you specify your own transaction
func (s *Store) TxInsert(tx *badger.Txn, key, data interface{}) error {
	storer := newStorer(data)
	var err error

	if _, ok := key.(sequence); ok {
		key, err = s.getSequence(storer.Type())
		if err != nil {
			return err
		}
	}

	gk, err := encodeKey(key, storer.Type())

	if err != nil {
		return err
	}

	_, err = tx.Get(gk)
	if err != badger.ErrKeyNotFound {
		return ErrKeyExists
	}

	value, err := encode(data)
	if err != nil {
		return err
	}

	// insert data
	err = tx.Set(gk, value)

	if err != nil {
		return err
	}

	// insert any indexes
	err = indexAdd(storer, tx, gk, data)
	if err != nil {
		return err
	}

	dataVal := reflect.Indirect(reflect.ValueOf(data))
	if !dataVal.CanSet() {
		return nil
	}
	dataType := dataVal.Type()

	for i := 0; i < dataType.NumField(); i++ {
		tf := dataType.Field(i)
		if _, ok := tf.Tag.Lookup(BadgerholdKeyTag); ok ||
			tf.Tag.Get(badgerholdPrefixTag) == badgerholdPrefixKeyValue {
			fieldValue := dataVal.Field(i)
			keyValue := reflect.ValueOf(key)
			if keyValue.Type() != tf.Type {
				break
			}
			if !fieldValue.CanSet() {
				break
			}
			if !reflect.DeepEqual(fieldValue.Interface(), reflect.Zero(tf.Type).Interface()) {
				break
			}
			fieldValue.Set(keyValue)
			break
		}
	}

	return nil
}

// Update updates an existing record in the badgerhold
// if the Key doesn't already exist in the store, then it fails with ErrNotFound
func (s *Store) Update(key interface{}, data interface{}) error {
	return s.Badger().Update(func(tx *badger.Txn) error {
		return s.TxUpdate(tx, key, data)
	})
}

// TxUpdate is the same as Update except it allows you to specify your own transaction
func (s *Store) TxUpdate(tx *badger.Txn, key interface{}, data interface{}) error {
	storer := newStorer(data)

	gk, err := encodeKey(key, storer.Type())

	if err != nil {
		return err
	}

	existingItem, err := tx.Get(gk)
	if err == badger.ErrKeyNotFound {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	// delete any existing indexes
	existingVal := reflect.New(reflect.TypeOf(data)).Interface()

	err = existingItem.Value(func(existing []byte) error {
		return decode(existing, existingVal)
	})
	if err != nil {
		return err
	}
	err = indexDelete(storer, tx, gk, existingVal)
	if err != nil {
		return err
	}

	value, err := encode(data)
	if err != nil {
		return err
	}

	// put data
	err = tx.Set(gk, value)
	if err != nil {
		return err
	}

	// insert any new indexes
	return indexAdd(storer, tx, gk, data)
}

// Upsert inserts the record into the badgerhold if it doesn't exist.  If it does already exist, then it updates
// the existing record
func (s *Store) Upsert(key interface{}, data interface{}) error {
	return s.Badger().Update(func(tx *badger.Txn) error {
		return s.TxUpsert(tx, key, data)
	})
}

// TxUpsert is the same as Upsert except it allows you to specify your own transaction
func (s *Store) TxUpsert(tx *badger.Txn, key interface{}, data interface{}) error {
	storer := newStorer(data)

	gk, err := encodeKey(key, storer.Type())

	if err != nil {
		return err
	}

	existingItem, err := tx.Get(gk)

	if err == nil {
		// existing entry found
		// delete any existing indexes
		existingVal := reflect.New(reflect.TypeOf(data)).Interface()

		err = existingItem.Value(func(existing []byte) error {
			return decode(existing, existingVal)
		})
		if err != nil {
			return err
		}

		err = indexDelete(storer, tx, gk, existingVal)
		if err != nil {
			return err
		}
	} else if err != badger.ErrKeyNotFound {
		return err
	}

	// existing entry not found

	value, err := encode(data)
	if err != nil {
		return err
	}

	// put data
	err = tx.Set(gk, value)
	if err != nil {
		return err
	}

	// insert any new indexes
	return indexAdd(storer, tx, gk, data)
}

// UpdateMatching runs the update function for every record that match the passed in query
// Note that the type  of record in the update func always has to be a pointer
func (s *Store) UpdateMatching(dataType interface{}, query *Query, update func(record interface{}) error) error {
	return s.Badger().Update(func(tx *badger.Txn) error {
		return s.TxUpdateMatching(tx, dataType, query, update)
	})
}

// TxUpdateMatching does the same as UpdateMatching, but allows you to specify your own transaction
func (s *Store) TxUpdateMatching(tx *badger.Txn, dataType interface{}, query *Query,
	update func(record interface{}) error) error {
	return updateQuery(tx, dataType, query, update)
}
