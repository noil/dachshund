package dachshund

type Pooler interface {
	Do(data interface{})
}
