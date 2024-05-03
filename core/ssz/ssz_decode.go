package ssz

import (
	"encoding/binary"
	"fmt"
	"reflect"
)

func UnmarshalSSZ(buf []byte, version int, schema ...any) (err error) {
	position := 0
	offsets := []int{}
	dynamicObjs := []SizedObjectSSZ{}

	// Iterate over each element in the schema
	for i, element := range schema {
		switch obj := element.(type) {
		case *uint64:
			if len(buf) < position+8 {
				return ErrLowBufferSize
			}
			// If the element is a pointer to uint64, decode it from the buf using little-endian encoding
			*obj = binary.LittleEndian.Uint64(buf[position:])
			position += 8
		case []byte:
			if len(buf) < position+len(obj) {
				return ErrLowBufferSize
			}
			// If the element is a byte slice, copy the corresponding data from the buf to the slice
			copy(obj, buf[position:])
			position += len(obj)
		case *byte:
			if len(buf) < position+1 {
				return ErrLowBufferSize
			}
			*obj = buf[position]
			position += 1
		case SizedObjectSSZ:
			// If the element implements the SizedObjectSSZ interface
			if obj.Static() {
				if len(buf) < position+obj.EncodingSizeSSZ() {
					return ErrLowBufferSize
				}
				// If the object is static (fixed size), decode it from the buf and update the position
				if err = obj.DecodeSSZ(buf[position:], version); err != nil {
					return fmt.Errorf("static element %d: %w", i, err)
				}
				position += obj.EncodingSizeSSZ()
			} else {
				if len(buf) < position+4 {
					return ErrLowBufferSize
				}
				// If the object is dynamic (variable size), store the offset and the object in separate slices
				offsets = append(offsets, int(binary.LittleEndian.Uint32(buf[position:])))
				dynamicObjs = append(dynamicObjs, obj)
				position += 4
			}
		default:
			// If the element does not match any supported types, throw panic, will be caught by anti-panic condom
			// and we will have the trace.
			panic(fmt.Errorf("RTFM, bad schema component %d", i))
		}
	}

	// Iterate over the dynamic objects and decode them using the stored offsets
	for i, obj := range dynamicObjs {
		endOffset := len(buf)
		if i != len(dynamicObjs)-1 {
			endOffset = offsets[i+1]
		}
		if offsets[i] > endOffset {
			return ErrBadOffset
		}
		if len(buf) < endOffset {
			return ErrLowBufferSize
		}
		if err = obj.DecodeSSZ(buf[offsets[i]:endOffset], version); err != nil {
			return fmt.Errorf("dynamic element (sz:%d) %d/%s: %w", endOffset-offsets[i], i, reflect.TypeOf(obj), err)
		}
	}

	return
}
