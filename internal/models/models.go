package models

type EnforceMode uint8

const (
	Log    EnforceMode = iota // 0: log violations, do not block
	Strict                    // 1: block unauthorized/rate-limited requests
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
