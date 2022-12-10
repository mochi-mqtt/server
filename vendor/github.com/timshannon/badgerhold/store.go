// Copyright 2019 Tim Shannon. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package badgerhold

import (
	"reflect"
	"strings"
	"sync"

	"github.com/dgraph-io/badger"
)

const (
	// BadgerHoldIndexTag is the struct tag used to define an a field as indexable for a badgerhold
	BadgerHoldIndexTag = "badgerholdIndex"

	// BadgerholdKeyTag is the struct tag used to define an a field as a key for use in a Find query
	BadgerholdKeyTag = "badgerholdKey"

	// badgerholdPrefixTag is the prefix for an alternate (more standard) version of a struct tag
	badgerholdPrefixTag         = "badgerhold"
	badgerholdPrefixIndexValue  = "index"
	badgerholdPrefixKeyValue    = "key"
	badgerholdPrefixUniqueValue = "unique"
)

// Store is a badgerhold wrapper around a badger DB
type Store struct {
	db               *badger.DB
	sequenceBandwith uint64
	sequences        *sync.Map
}

// Options allows you set different options from the defaults
// For example the encoding and decoding funcs which default to Gob
type Options struct {
	Encoder          EncodeFunc
	Decoder          DecodeFunc
	SequenceBandwith uint64
	badger.Options
}

// DefaultOptions are a default set of options for opening a BadgerHold database
// Includes badgers own default options
var DefaultOptions = Options{
	Options:          badger.DefaultOptions(""),
	Encoder:          DefaultEncode,
	Decoder:          DefaultDecode,
	SequenceBandwith: 100,
}

// Open opens or creates a badgerhold file.
func Open(options Options) (*Store, error) {
	encode = options.Encoder
	decode = options.Decoder

	db, err := badger.Open(options.Options)
	if err != nil {
		return nil, err
	}

	return &Store{
		db:               db,
		sequenceBandwith: options.SequenceBandwith,
		sequences:        &sync.Map{},
	}, nil
}

// Badger returns the underlying Badger DB the badgerhold is based on
func (s *Store) Badger() *badger.DB {
	return s.db
}

// Close closes the badger db
func (s *Store) Close() error {
	var err error
	s.sequences.Range(func(key, value interface{}) bool {
		err = value.(*badger.Sequence).Release()
		if err != nil {
			return false
		}
		return true
	})
	if err != nil {
		return err
	}
	return s.db.Close()
}

/*
	NOTE: Not going to implement ReIndex and Remove index
	I had originally created these to make the transition from a plain bolt or badger DB easier
	but there is too much chance for lost data, and it's probably better that any conversion be
	done by the developer so they can directly manage how they want data to be migrated.
	If you disagree, feel free to open an issue and we can revisit this.
*/

// Storer is the Interface to implement to skip reflect calls on all data passed into the badgerhold
type Storer interface {
	Type() string              // used as the badgerdb index prefix
	Indexes() map[string]Index //[indexname]indexFunc
}

// anonType is created from a reflection of an unknown interface
type anonStorer struct {
	rType   reflect.Type
	indexes map[string]Index
}

// Type returns the name of the type as determined from the reflect package
func (t *anonStorer) Type() string {
	return t.rType.Name()
}

// Indexes returns the Indexes determined by the reflect package on this type
func (t *anonStorer) Indexes() map[string]Index {
	return t.indexes
}

// newStorer creates a type which satisfies the Storer interface based on reflection of the passed in dataType
// if the Type doesn't meet the requirements of a Storer (i.e. doesn't have a name) it panics
// You can avoid any reflection costs, by implementing the Storer interface on a type
func newStorer(dataType interface{}) Storer {
	s, ok := dataType.(Storer)

	if ok {
		return s
	}

	tp := reflect.TypeOf(dataType)

	for tp.Kind() == reflect.Ptr {
		tp = tp.Elem()
	}

	storer := &anonStorer{
		rType:   tp,
		indexes: make(map[string]Index),
	}

	if storer.rType.Name() == "" {
		panic("Invalid Type for Storer.  Type is unnamed")
	}

	if storer.rType.Kind() != reflect.Struct {
		panic("Invalid Type for Storer.  BadgerHold only works with structs")
	}

	for i := 0; i < storer.rType.NumField(); i++ {

		indexName := ""
		unique := false

		if strings.Contains(string(storer.rType.Field(i).Tag), BadgerHoldIndexTag) {
			indexName = storer.rType.Field(i).Tag.Get(BadgerHoldIndexTag)

			if indexName != "" {
				indexName = storer.rType.Field(i).Name
			}
		} else if tag := storer.rType.Field(i).Tag.Get(badgerholdPrefixTag); tag != "" {
			if tag == badgerholdPrefixIndexValue {
				indexName = storer.rType.Field(i).Name
			} else if tag == badgerholdPrefixUniqueValue {
				indexName = storer.rType.Field(i).Name
				unique = true
			}
		}

		if indexName != "" {
			storer.indexes[indexName] = Index{
				IndexFunc: func(name string, value interface{}) ([]byte, error) {
					tp := reflect.ValueOf(value)
					for tp.Kind() == reflect.Ptr {
						tp = tp.Elem()
					}

					return encode(tp.FieldByName(name).Interface())
				},
				Unique: unique,
			}
		}
	}

	return storer
}

func (s *Store) getSequence(typeName string) (uint64, error) {
	seq, ok := s.sequences.Load(typeName)
	if !ok {
		newSeq, err := s.Badger().GetSequence([]byte(typeName), s.sequenceBandwith)
		if err != nil {
			return 0, err
		}
		s.sequences.Store(typeName, newSeq)
		seq = newSeq
	}

	return seq.(*badger.Sequence).Next()
}

func typePrefix(typeName string) []byte {
	return []byte("bh_" + typeName)
}
