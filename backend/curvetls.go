package backend

import (
	"github.com/Rudd-O/curvetls"
	"github.com/asdine/storm"
)

type StormKeystore struct {
	DB *storm.DB
}

func (sk *StormKeystore) Allowed(pubkey curvetls.Pubkey) bool {
	var kp KeyPair
	// pass the value we are querying for as the second param
	if err := sk.DB.One("Pub", pubkey.String(), &kp); err == nil {
		return true
	}
	return false
}
