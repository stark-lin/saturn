// This file contains immutable Accounting transaction behavior helpers.
package accounting

func (t Transaction) SignedAmount() int64 {
	return t.AmountCents
}
