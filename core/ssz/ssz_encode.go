package ssz

import (
	"encoding/binary"
	"fmt"
)

func MarshalSSZ(dst *[]byte, schema ...any) (err error) {
	defer func() {
		if err2 := recover(); err2 != nil {
			err = fmt.Errorf("panic while encoding: %v", err2)
		}
	}()

	if dst == nil {
		panic("cannot serialize value to nil buffer")
	}

	if *dst == nil {
		*dst = make([]byte, 0)
	}

	currentOffset := 0
	dynamicComponents := []SizedObjectSSZ{}
	offsetsStarts := []int{}

	// Iterate over each element in the schema
	for i, element := range schema {
		switch obj := element.(type) {
		case uint64:
			// If the element is a uint64, encode it using SSZ and append it to the dst
			*dst = append(*dst, Uint64SSZ(obj)...)
			currentOffset += 8
		case *uint64:
			// If the element is a pointer to uint64, dereference it, encode it using SSZ, and append it to the dst
			*dst = append(*dst, Uint64SSZ(*obj)...)
			currentOffset += 8
		case []byte:
			// If the element is a byte slice, append it to the dst
			*dst = append(*dst, obj...)
			currentOffset += len(obj)
		case SizedObjectSSZ:
			// If the element implements the SizedObjectSSZ interface
			startSize := len(*dst)
			if obj.Static() {
				// If the object is static (fixed size), encode it using SSZ and update the dst
				if err = obj.EncodeSSZ(dst); err != nil {
					return err
				}
			} else {
				// If the object is dynamic (variable size), store the start offset and the object in separate slices
				offsetsStarts = append(offsetsStarts, startSize)
				*dst = append(*dst, make([]byte, 4)...)
				dynamicComponents = append(dynamicComponents, obj)
			}
			currentOffset += len(*dst) - startSize
		default:
			// If the element does not match any supported types, panic with an error message
			panic(fmt.Sprintf("u must suffer from dementia, pls read the doc of this method (aka. comments), bad schema component %d", i))
		}
	}

	// Iterate over the dynamic components and encode them using SSZ
	for i, dynamicComponent := range dynamicComponents {
		startSize := len(*dst)
		binary.LittleEndian.PutUint32((*dst)[offsetsStarts[i]:], uint32(currentOffset))

		if err = dynamicComponent.EncodeSSZ(dst); err != nil {
			return err
		}
		currentOffset += len(*dst) - startSize
	}

	return nil
}
