// Copyright 2019 Tim Shannon. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package badgerhold

import (
	"reflect"

	"github.com/dgraph-io/badger"
)

// Delete deletes a record from the bolthold, datatype just needs to be an example of the type stored so that
// the proper bucket and indexes are updated
func (s *Store) Delete(key, dataType interface{}) error {
	return s.Badger().Update(func(tx *badger.Txn) error {
		return s.TxDelete(tx, key, dataType)
	})
}

// TxDelete is the same as Delete except it allows you specify your own transaction
func (s *Store) TxDelete(tx *badger.Txn, key, dataType interface{}) error {
	storer := newStorer(dataType)
	gk, err := encodeKey(key, storer.Type())

	if err != nil {
		return err
	}

	value := reflect.New(reflect.TypeOf(dataType)).Interface()

	item, err := tx.Get(gk)
	if err == badger.ErrKeyNotFound {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	item.Value(func(bVal []byte) error {
		return decode(bVal, value)
	})
	if err != nil {
		return err
	}

	// delete data
	err = tx.Delete(gk)

	if err != nil {
		return err
	}

	// remove any indexes
	return indexDelete(storer, tx, gk, value)
}

// DeleteMatching deletes all of the records that match the passed in query
func (s *Store) DeleteMatching(dataType interface{}, query *Query) error {
	return s.Badger().Update(func(tx *badger.Txn) error {
		return s.TxDeleteMatching(tx, dataType, query)
	})
}

// TxDeleteMatching does the same as DeleteMatching, but allows you to specify your own transaction
func (s *Store) TxDeleteMatching(tx *badger.Txn, dataType interface{}, query *Query) error {
	return deleteQuery(tx, dataType, query)
}
