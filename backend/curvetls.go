package backend

import (
	"github.com/Rudd-O/curvetls"
	"github.com/asdine/storm"
)

type StormKeystore struct {
	DB *storm.DB
}

func (sk *StormKeystore) Allowed(pubkey curvetls.Pubkey) bool {
	var pk curvetls.Pubkey
	if err := sk.DB.One("Pubkey", &pubkey, &pk); err == nil {
		return true
	}
	return false
}
