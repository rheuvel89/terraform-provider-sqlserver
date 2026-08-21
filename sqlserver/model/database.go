package model

type Database struct {
	Name               string
	Owner              string
	Collation          string
	RecoveryModel      string
	CompatibilityLevel int
}
