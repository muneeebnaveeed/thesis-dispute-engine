// Package money formats amounts at a currency's minor unit, which is what people, statements and the API's
// consumers expect; storage keeps four places so no rounding happens before the customer sees a number.
package money

import "github.com/shopspring/decimal"

// minorUnits are the ISO 4217 exponents that differ from two.
var minorUnits = map[string]int32{"HUF": 0, "JPY": 0, "KRW": 0, "ISK": 0, "CLP": 0, "BHD": 3, "KWD": 3, "JOD": 3, "OMR": 3, "TND": 3}

// Minor is the number of decimal places a currency is quoted in.
func Minor(currency string) int32 {
	if n, ok := minorUnits[currency]; ok {
		return n
	}
	return 2
}

// Format renders an amount at the currency's minor unit, e.g. "1899.00" for EUR, "1899" for HUF.
func Format(d decimal.Decimal, currency string) string {
	return d.StringFixed(Minor(currency))
}
