package rpc

import (
	"io"
	"reflect"

	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/kkharji/bel"
)

func ExportTypescriptTypes(out io.Writer) {
	// reflect on jsonrpc.EthAPI to generate typescript types for it iterate trough all methods
	ethAPIType := reflect.TypeOf((*jsonrpc.EthAPI)(nil)).Elem()
	ts := make([]bel.TypescriptType, 0)

	for i := 0; i < ethAPIType.NumMethod(); i++ {
		method := ethAPIType.Method(i)
		for j := 1; j < method.Type.NumIn(); j++ {
			paramType := method.Type.In(j)
			if paramType.Kind() == reflect.Struct {
				tsAdd, err := bel.Extract(reflect.Zero(paramType).Interface())
				if err != nil {
					panic(err)
				}
				ts = append(ts, tsAdd...)
			}
		}
		output := method.Type.Out(0)
		if output.Kind() == reflect.Struct {
			tsAdd, err := bel.Extract(reflect.Zero(output).Interface())
			if err != nil {
				panic(err)
			}
			ts = append(ts, tsAdd...)
		}

	}

	tsAdd, err := bel.Extract((*jsonrpc.EthAPI)(nil))

	if err != nil {
		panic(err)
	}

	ts = append(ts, tsAdd...)

	err = bel.Render(ts, bel.GenerateOutputTo(out))
	if err != nil {
		panic(err)
	}
}
