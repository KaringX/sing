package badoption

import (
	"context"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

type Listable[T any] []T

func (l Listable[T]) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	arrayList := []T(l)
	if len(arrayList) == 1 {
		return json.Marshal(arrayList[0])
	}
	return json.MarshalContext(ctx, arrayList)
}

func (l *Listable[T]) UnmarshalJSONContext(ctx context.Context, content []byte) error {
	if string(content) == "null" {
		return nil
	}
	var singleItem T
	err := json.UnmarshalContextAllowUnknownFields(ctx, content, &singleItem) //karing
	if err == nil {
		*l = []T{singleItem}
		return nil
	}
	newErr := json.UnmarshalContextAllowUnknownFields(ctx, content, (*[]T)(l))  //karing
	if newErr == nil {
		return nil
	}
	return E.Errors(err, newErr)
}
