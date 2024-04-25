// Package encode provides methods to convert values to and from kRPC's protobuf format.
package encode

import (
	"math"
	"reflect"

	"github.com/atburke/krpc-go/lib/service"
	"github.com/atburke/krpc-go/types"
	"github.com/golang/protobuf/proto"
	"github.com/ztrue/tracerr"
)

// isEmptyStruct checks if a type represents an empty struct.
func isEmptyStruct(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.NumField() == 0
}

// Marshal encodes a type in kRPC's protobuf format.
func Marshal(m interface{}) ([]byte, error) {
	var err error
	buf := proto.NewBuffer([]byte{})
	var b []byte
	switch v := m.(type) {
	// Special types
	case proto.Message:
		b, err = proto.Marshal(v)
	case service.Class:
		b, err = Marshal(v.ID_internal())
	case service.Enum:
		b, err = Marshal(v.Value())
	// Varints
	case int32:
		err = buf.EncodeZigzag32(uint64(v))
	case int64:
		err = buf.EncodeZigzag64(uint64(v))
	case uint32:
		err = buf.EncodeVarint(uint64(v))
	case uint64:
		err = buf.EncodeVarint(v)
	case bool:
		var u uint64
		if v {
			u = 1
		}
		err = buf.EncodeVarint(u)
	// Floats
	case float32:
		err = buf.EncodeFixed32(uint64(math.Float32bits(v)))
	case float64:
		err = buf.EncodeFixed64(math.Float64bits(v))
	// Strings and bytes
	case string:
		err = buf.EncodeStringBytes(v)
	case []byte:
		err = buf.EncodeRawBytes(v)
	}

	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	if b != nil {
		return b, nil
	}
	if len(buf.Bytes()) > 0 {
		return buf.Bytes(), nil
	}

	// We have to use reflection for collections.
	value := reflect.ValueOf(m)
	mType := reflect.TypeOf(m)
	switch mType.Kind() {
	case reflect.Slice:
		var list types.List
		for i := 0; i < value.Len(); i++ {
			bb, err := Marshal(value.Index(i).Interface())
			if err != nil {
				return nil, tracerr.Wrap(err)
			}
			list.Items = append(list.Items, bb)
		}
		b, err = proto.Marshal(&list)
	case reflect.Map:
		elemType := mType.Elem()
		// m is a Set (has value type struct{})
		if isEmptyStruct(elemType) {
			var set types.Set
			iter := value.MapRange()
			for iter.Next() {
				itemBytes, err := Marshal(iter.Key().Interface())
				if err != nil {
					return nil, tracerr.Wrap(err)
				}
				set.Items = append(set.Items, itemBytes)
			}
			b, err = proto.Marshal(&set)
			// m is a Dictionary
		} else {
			var dict types.Dictionary
			iter := value.MapRange()
			for iter.Next() {
				keyBytes, err := Marshal(iter.Key().Interface())
				if err != nil {
					return nil, tracerr.Wrap(err)
				}
				valueBytes, err := Marshal(iter.Value().Interface())
				if err != nil {
					return nil, tracerr.Wrap(err)
				}
				dict.Entries = append(dict.Entries, &types.DictionaryEntry{
					Key:   keyBytes,
					Value: valueBytes,
				})
			}
			b, err = proto.Marshal(&dict)
		}
		// Assume it's a Tuple
	case reflect.Struct:
		var tuple types.Tuple
		for i := 0; i < mType.NumField(); i++ {
			fieldBytes, err := Marshal(value.Field(i).Interface())
			if err != nil {
				return nil, tracerr.Wrap(err)
			}
			tuple.Items = append(tuple.Items, fieldBytes)
		}
		b, err = proto.Marshal(&tuple)
	case reflect.Pointer:
		if mType.Elem().Kind() != reflect.Pointer {
			b, err := Marshal(value.Elem().Interface())
			return b, tracerr.Wrap(err)
		}
		fallthrough
	default:
		return nil, tracerr.Errorf("Unsupported type: %v", reflect.TypeOf(m))
	}

	return b, tracerr.Wrap(err)
}

// Unmarshal decodes a type from kRPC's protobuf format.
func Unmarshal(b []byte, m interface{}) error {
	buf := proto.NewBuffer(b)
	var err error
	var u uint64
	var isCollection bool
	switch v := m.(type) {
	// Special types
	case proto.Message:
		err = proto.Unmarshal(b, v)
	case service.Class:
		err = Unmarshal(b, &u)
		if err == nil {
			v.SetID_internal(u)
		}
	case service.SettableEnum:
		var value int32
		err = Unmarshal(b, &value)
		if err == nil {
			v.SetValue(value)
		}
	// Varints
	case *int32:
		u, err = buf.DecodeZigzag32()
		if err == nil {
			*v = int32(u)
		}
	case *int64:
		u, err = buf.DecodeZigzag64()
		if err == nil {
			*v = int64(u)
		}
	case *uint32:
		u, err = buf.DecodeVarint()
		if err == nil {
			*v = uint32(u)
		}
	case *uint64:
		u, err = buf.DecodeVarint()
		if err == nil {
			*v = u
		}
	case *bool:
		u, err = buf.DecodeVarint()
		if err == nil {
			*v = (u != 0)
		}
	// Floats
	case *float32:
		u, err = buf.DecodeFixed32()
		if err == nil {
			*v = math.Float32frombits(uint32(u))
		}
	case *float64:
		u, err = buf.DecodeFixed64()
		if err == nil {
			*v = math.Float64frombits(u)
		}
	// Strings and bytes
	case *string:
		var s string
		s, err = buf.DecodeStringBytes()
		if err == nil {
			*v = s
		}
	case *[]byte:
		var bb []byte
		bb, err = buf.DecodeRawBytes(false)
		if err == nil {
			*v = bb
		}
	default:
		isCollection = true
	}

	if !isCollection {
		return tracerr.Wrap(err)
	}

	mType := reflect.TypeOf(m)
	if mType.Kind() != reflect.Pointer {
		return tracerr.Errorf("Message arg is not a pointer")
	}

	mInternalType := mType.Elem()
	switch mInternalType.Kind() {
	case reflect.Slice:
		var list types.List
		if err := proto.Unmarshal(b, &list); err != nil {
			return tracerr.Wrap(err)
		}
		elemType := mInternalType.Elem()
		slice := reflect.MakeSlice(mInternalType, 0, cap(list.Items))
		for _, elemBytes := range list.Items {

			var elem reflect.Value
			if elemType.Kind() == reflect.Pointer {
				elem = reflect.New(elemType.Elem())
			} else {
				elem = reflect.New(elemType)
			}

			if err := Unmarshal(elemBytes, elem.Interface()); err != nil {
				return tracerr.Wrap(err)
			}

			out := elem
			if elemType.Kind() != reflect.Pointer {
				out = out.Elem()
			}
			slice = reflect.Append(slice, out)
		}
		reflect.ValueOf(m).Elem().Set(slice)
	case reflect.Map:
		keyType := mInternalType.Key()
		elemType := mInternalType.Elem()
		// Set
		if isEmptyStruct(elemType) {
			var set types.Set
			if err := proto.Unmarshal(b, &set); err != nil {
				return tracerr.Wrap(err)
			}
			setMap := reflect.MakeMap(mInternalType)
			for _, elemBytes := range set.Items {
				elem := reflect.New(keyType)
				if err := Unmarshal(elemBytes, elem.Interface()); err != nil {
					return tracerr.Wrap(err)
				}
				setMap.SetMapIndex(elem.Elem(), reflect.Zero(elemType))
			}
			reflect.ValueOf(m).Elem().Set(setMap)
			// Dictionary
		} else {
			var dict types.Dictionary
			if err := proto.Unmarshal(b, &dict); err != nil {
				return tracerr.Wrap(err)
			}
			dictMap := reflect.MakeMap(mInternalType)
			for _, entry := range dict.Entries {
				key := reflect.New(keyType)
				if err := Unmarshal(entry.Key, key.Interface()); err != nil {
					return tracerr.Wrap(err)
				}

				var value reflect.Value
				if elemType.Kind() == reflect.Pointer {
					value = reflect.New(elemType.Elem())
				} else {
					value = reflect.New(elemType)
				}

				if err := Unmarshal(entry.Value, value.Interface()); err != nil {
					return tracerr.Wrap(err)
				}

				out := value
				if elemType.Kind() != reflect.Pointer {
					out = out.Elem()
				}
				dictMap.SetMapIndex(key.Elem(), out)
			}
			reflect.ValueOf(m).Elem().Set(dictMap)
		}
	case reflect.Struct:
		var tuple types.Tuple
		if err := proto.Unmarshal(b, &tuple); err != nil {
			return tracerr.Wrap(err)
		}
		if len(tuple.Items) != mInternalType.NumField() {
			return tracerr.Errorf("Wrong tuple type; expected %v elements", len(tuple.Items))
		}
		tupleStruct := reflect.New(mInternalType).Elem()
		for i, elemBytes := range tuple.Items {
			elem := reflect.New(tupleStruct.Field(i).Type())
			if err := Unmarshal(elemBytes, elem.Interface()); err != nil {
				return tracerr.Wrap(err)
			}
			tupleStruct.Field(i).Set(elem.Elem())
		}
		reflect.ValueOf(m).Elem().Set(tupleStruct)
	default:
		return tracerr.Errorf("Unsupported type: %v", mType)
	}

	return tracerr.Wrap(err)
}
