package models

type EnforceMode uint8

const (
	Strict EnforceMode = iota
	Log
)

func (em EnforceMode) String() string {
	switch em {
	case Strict:
		return "strict"
	case Log:
		return "log"
	default:
		return "invalid"
	}
}
