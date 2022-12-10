// Copyright 2019 Tim Shannon. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package badgerhold

import (
	"bytes"
	"encoding/gob"
)

// EncodeFunc is a function for encoding a value into bytes
type EncodeFunc func(value interface{}) ([]byte, error)

// DecodeFunc is a function for decoding a value from bytes
type DecodeFunc func(data []byte, value interface{}) error

var encode EncodeFunc
var decode DecodeFunc

// DefaultEncode is the default encoding func for badgerhold (Gob)
func DefaultEncode(value interface{}) ([]byte, error) {
	var buff bytes.Buffer

	en := gob.NewEncoder(&buff)

	err := en.Encode(value)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// DefaultDecode is the default decoding func for badgerhold (Gob)
func DefaultDecode(data []byte, value interface{}) error {
	var buff bytes.Buffer
	de := gob.NewDecoder(&buff)

	_, err := buff.Write(data)
	if err != nil {
		return err
	}

	return de.Decode(value)
}

// encodeKey encodes key values with a type prefix which allows multiple different types
// to exist in the badger DB
func encodeKey(key interface{}, typeName string) ([]byte, error) {
	encoded, err := encode(key)
	if err != nil {
		return nil, err
	}

	return append(typePrefix(typeName), encoded...), nil
}

// decodeKey decodes the key value and removes the type prefix
func decodeKey(data []byte, key interface{}, typeName string) error {
	return decode(data[len(typePrefix(typeName)):], key)
}
