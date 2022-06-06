package proto

func NewMemosphere() *Memosphere {
	return &Memosphere{
		Data:    map[string]*Memos{},
		Modules: map[string]*Thunk{},
	}
}
