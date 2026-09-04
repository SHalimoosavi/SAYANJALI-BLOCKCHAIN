package state

import "errors"

type Balances map[string]uint64

func (b Balances) Credit(addr string, amount uint64) error {
	cur := b[addr]
	if ^uint64(0)-cur < amount {
		return errors.New("balance overflow")
	}
	b[addr] = cur + amount
	return nil
}
func (b Balances) Debit(addr string, amount uint64) error {
	cur := b[addr]
	if amount > cur {
		return errors.New("insufficient balance")
	}
	b[addr] = cur - amount
	return nil
}
