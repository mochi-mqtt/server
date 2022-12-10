// Copyright 2019 Tim Shannon. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package badgerhold

import (
	"bytes"
	"reflect"
	"sort"

	"github.com/dgraph-io/badger"
)

const indexPrefix = "_bhIndex"

// size of iterator keys stored in memory before more are fetched
const iteratorKeyMinCacheSize = 100

// Index is a function that returns the indexable, encoded bytes of the passed in value
type Index struct {
	IndexFunc func(name string, value interface{}) ([]byte, error)
	Unique    bool
}

// adds an item to the index
func indexAdd(storer Storer, tx *badger.Txn, key []byte, data interface{}) error {
	indexes := storer.Indexes()
	for name, index := range indexes {
		err := indexUpdate(storer.Type(), name, index, tx, key, data, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// // removes an item from the index
// // be sure to pass the data from the old record, not the new one
func indexDelete(storer Storer, tx *badger.Txn, key []byte, originalData interface{}) error {
	indexes := storer.Indexes()

	for name, index := range indexes {
		err := indexUpdate(storer.Type(), name, index, tx, key, originalData, true)
		if err != nil {
			return err
		}
	}

	return nil
}

// // adds or removes a specific index on an item
func indexUpdate(typeName, indexName string, index Index, tx *badger.Txn, key []byte, value interface{},
	delete bool) error {

	indexKey, err := index.IndexFunc(indexName, value)
	if indexKey == nil {
		return nil
	}

	indexValue := make(keyList, 0)

	if err != nil {
		return err
	}

	indexKey = append(indexKeyPrefix(typeName, indexName), indexKey...)

	item, err := tx.Get(indexKey)
	if err != nil && err != badger.ErrKeyNotFound {
		return err
	}

	if err != badger.ErrKeyNotFound {
		if index.Unique && !delete {
			return ErrUniqueExists
		}
		err = item.Value(func(iVal []byte) error {
			return decode(iVal, &indexValue)
		})
		if err != nil {
			return err
		}
	}

	if delete {
		indexValue.remove(key)
	} else {
		indexValue.add(key)
	}

	if len(indexValue) == 0 {
		return tx.Delete(indexKey)
	}

	iVal, err := encode(indexValue)
	if err != nil {
		return err
	}

	return tx.Set(indexKey, iVal)
}

// indexKeyPrefix returns the prefix of the badger key where this index is stored
func indexKeyPrefix(typeName, indexName string) []byte {
	return []byte(indexPrefix + ":" + typeName + ":" + indexName)
}

// keyList is a slice of unique, sorted keys([]byte) such as what an index points to
type keyList [][]byte

func (v *keyList) add(key []byte) {
	i := sort.Search(len(*v), func(i int) bool {
		return bytes.Compare((*v)[i], key) >= 0
	})

	if i < len(*v) && bytes.Equal((*v)[i], key) {
		// already added
		return
	}

	*v = append(*v, nil)
	copy((*v)[i+1:], (*v)[i:])
	(*v)[i] = key
}

func (v *keyList) remove(key []byte) {
	i := sort.Search(len(*v), func(i int) bool {
		return bytes.Compare((*v)[i], key) >= 0
	})

	if i < len(*v) {
		copy((*v)[i:], (*v)[i+1:])
		(*v)[len(*v)-1] = nil
		*v = (*v)[:len(*v)-1]
	}
}

func (v *keyList) in(key []byte) bool {
	i := sort.Search(len(*v), func(i int) bool {
		return bytes.Compare((*v)[i], key) >= 0
	})

	return (i < len(*v) && bytes.Equal((*v)[i], key))
}

func indexExists(it *badger.Iterator, typeName, indexName string) bool {
	iPrefix := indexKeyPrefix(typeName, indexName)
	tPrefix := typePrefix(typeName)
	// test if any data exists for type
	it.Seek(tPrefix)
	if !it.ValidForPrefix(tPrefix) {
		// store is empty for this data type so the index could possibly exist
		// we don't want to fail on a "bad index" because they could simply be running a query against
		// an empty dataset
		return true
	}

	// test if an index exists
	it.Seek(iPrefix)
	if it.ValidForPrefix(iPrefix) {
		return true
	}

	return false
}

type iterator struct {
	keyCache [][]byte
	nextKeys func(*badger.Iterator) ([][]byte, error)
	iter     *badger.Iterator
	bookmark *iterBookmark
	lastSeek []byte
	tx       *badger.Txn
	err      error
}

// iterBookmark stores a seek location in a specific iterator
// so that a single RW iterator can be shared within a single transaction
type iterBookmark struct {
	iter    *badger.Iterator
	seekKey []byte
}

func newIterator(tx *badger.Txn, typeName string, query *Query, bookmark *iterBookmark) *iterator {
	i := &iterator{
		tx: tx,
	}

	if bookmark != nil {
		i.iter = bookmark.iter
	} else {
		i.iter = tx.NewIterator(badger.DefaultIteratorOptions)
	}

	var prefix []byte

	if query.index != "" {
		query.badIndex = !indexExists(i.iter, typeName, query.index)
	}

	criteria := query.fieldCriteria[query.index]
	if hasMatchFunc(criteria) {
		// can't use indexes on matchFuncs as the entire record isn't available for testing in the passed
		// in function
		criteria = nil
	}

	// Key field or index not specified - test key against criteria (if it exists) or return everything
	if query.index == "" || len(criteria) == 0 {
		prefix = typePrefix(typeName)
		i.iter.Seek(prefix)
		i.nextKeys = func(iter *badger.Iterator) ([][]byte, error) {
			var nKeys [][]byte

			for len(nKeys) < iteratorKeyMinCacheSize {
				if !iter.ValidForPrefix(prefix) {
					return nKeys, nil
				}

				item := iter.Item()
				key := item.KeyCopy(nil)
				var ok bool
				if len(criteria) == 0 {
					// nothing to check return key for value testing
					ok = true
				} else {

					val := reflect.New(query.dataType)

					err := item.Value(func(v []byte) error {
						return decode(v, val.Interface())
					})
					if err != nil {
						return nil, err
					}

					ok, err = matchesAllCriteria(criteria, key, true, typeName, val.Interface())
					if err != nil {
						return nil, err
					}
				}

				if ok {
					nKeys = append(nKeys, key)

				}
				i.lastSeek = key
				iter.Next()
			}
			return nKeys, nil
		}

		return i
	}

	// indexed field, get keys from index
	prefix = indexKeyPrefix(typeName, query.index)
	i.iter.Seek(prefix)
	i.nextKeys = func(iter *badger.Iterator) ([][]byte, error) {
		var nKeys [][]byte

		for len(nKeys) < iteratorKeyMinCacheSize {
			if !iter.ValidForPrefix(prefix) {
				return nKeys, nil
			}

			item := iter.Item()
			key := item.KeyCopy(nil)
			// no currentRow on indexes as it refers to multiple rows
			// remove index prefix for matching
			ok, err := matchesAllCriteria(criteria, key[len(prefix):], true, "", nil)
			if err != nil {
				return nil, err
			}

			if ok {
				item.Value(func(v []byte) error {
					// append the slice of keys stored in the index
					var keys = make(keyList, 0)
					err := decode(v, &keys)
					if err != nil {
						return err
					}

					nKeys = append(nKeys, [][]byte(keys)...)
					return nil
				})
			}

			i.lastSeek = key
			iter.Next()

		}
		return nKeys, nil

	}

	return i
}

func (i *iterator) createBookmark() *iterBookmark {
	return &iterBookmark{
		iter:    i.iter,
		seekKey: i.lastSeek,
	}
}

// Next returns the next key value that matches the iterators criteria
// If no more kv's are available the return nil, if there is an error, they return nil
// and iterator.Error() will return the error
func (i *iterator) Next() (key []byte, value []byte) {
	if i.err != nil {
		return nil, nil
	}

	if len(i.keyCache) == 0 {
		newKeys, err := i.nextKeys(i.iter)
		if err != nil {
			i.err = err
			return nil, nil
		}

		if len(newKeys) == 0 {
			return nil, nil
		}

		i.keyCache = append(i.keyCache, newKeys...)
	}

	key = i.keyCache[0]
	i.keyCache = i.keyCache[1:]

	item, err := i.tx.Get(key)
	if err != nil {
		i.err = err
		return nil, nil
	}

	err = item.Value(func(val []byte) error {
		value = val
		return nil
	})
	if err != nil {
		i.err = err
		return nil, nil
	}

	return
}

// Error returns the last error, iterator.Next() will not continue if there is an error present
func (i *iterator) Error() error {
	return i.err
}

func (i *iterator) Close() {
	if i.bookmark != nil {
		i.iter.Seek(i.bookmark.seekKey)
		return
	}

	i.iter.Close()
}
