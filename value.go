package bass

type Value interface {
	Decode(interface{}) error
}
