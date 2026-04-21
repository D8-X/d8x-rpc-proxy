package models

type EnforceMode uint8

const (
	Strict EnforceMode = iota
	Log
)
