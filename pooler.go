package dachshund

type Pooler interface {
	Do(data interface{})
	Release()
	Reload(number int)
}
