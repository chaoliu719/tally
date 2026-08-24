// Package currency provides a static reference table of the ISO 4217
// currency codes tally supports, and how many digits follow the decimal
// point in each one's smallest unit. It replaces ezbookkeeping's currency
// validator, which supports the same set of real-world currencies but
// (incorrectly, for tally's purposes) treats every one of them as having
// two decimal digits.
//
// This is static, hand-maintained Go data, not a database table: the set of
// circulating ISO 4217 currencies changes rarely, and nothing here needs to
// be queried with SQL.
package currency

// Decimals reports the number of digits after the decimal point in code's
// smallest unit (e.g. 2 for "USD", 0 for "JPY", 3 for "BHD"), and whether
// code is a supported ISO 4217 currency code at all.
func Decimals(code string) (digits int, ok bool) {
	digits, ok = decimalDigits[code]
	return digits, ok
}

// Supported reports whether code is a supported ISO 4217 currency code.
func Supported(code string) bool {
	_, ok := decimalDigits[code]
	return ok
}

var decimalDigits = buildDecimalDigits()

func buildDecimalDigits() map[string]int {
	digits := make(map[string]int, len(circulatingCodes))
	for _, code := range circulatingCodes {
		digits[code] = 2
	}
	for _, code := range zeroDecimalCodes {
		digits[code] = 0
	}
	for _, code := range threeDecimalCodes {
		digits[code] = 3
	}
	return digits
}

// zeroDecimalCodes lists ISO 4217 currencies whose smallest unit has no
// fractional part (e.g. the Japanese yen has no subunit in practical use).
var zeroDecimalCodes = []string{
	"BIF", "CLP", "DJF", "GNF", "ISK", "JPY", "KMF", "KRW",
	"MGA", "MRU", "PYG", "RWF", "UGX", "VND", "VUV", "XAF", "XOF", "XPF",
}

// threeDecimalCodes lists ISO 4217 currencies whose smallest unit is one
// thousandth of the major unit (e.g. the Bahraini dinar is subdivided into
// 1000 fils).
var threeDecimalCodes = []string{"BHD", "IQD", "JOD", "KWD", "LYD", "OMR", "TND"}

// circulatingCodes lists every ISO 4217 currency code tally supports.
// Decimal digits default to 2 unless overridden by zeroDecimalCodes or
// threeDecimalCodes above. This intentionally excludes non-circulating
// codes with no conventional decimal treatment (precious metals XAU/XAG/
// XPD/XPT, the SDR and related clearing units XDR/XUA/XSU/XBA-XBD, the test
// code XTS, and the "no currency" code XXX).
var circulatingCodes = []string{
	"AED", "AFN", "ALL", "AMD", "ANG", "AOA", "ARS", "AUD", "AWG", "AZN",
	"BAM", "BBD", "BDT", "BGN", "BMD", "BND", "BOB", "BRL", "BSD", "BTN", "BWP", "BYN", "BZD",
	"CAD", "CDF", "CHF", "CNY", "COP", "CRC", "CUP", "CVE", "CZK",
	"DKK", "DOP", "DZD",
	"EGP", "ERN", "ETB", "EUR",
	"FJD", "FKP",
	"GBP", "GEL", "GHS", "GIP", "GMD", "GTQ", "GYD",
	"HKD", "HNL", "HTG", "HUF",
	"IDR", "ILS", "INR", "IRR",
	"JMD",
	"KES", "KGS", "KHR", "KPW", "KYD", "KZT",
	"LAK", "LBP", "LKR", "LRD", "LSL",
	"MAD", "MDL", "MKD", "MMK", "MNT", "MOP", "MUR", "MVR", "MWK", "MXN", "MYR", "MZN",
	"NAD", "NGN", "NIO", "NOK", "NPR", "NZD",
	"PAB", "PEN", "PGK", "PHP", "PKR", "PLN",
	"QAR",
	"RON", "RSD", "RUB",
	"SAR", "SBD", "SCR", "SDG", "SEK", "SGD", "SHP", "SLE", "SOS", "SRD", "SSP", "STN", "SVC", "SYP", "SZL",
	"THB", "TJS", "TMT", "TOP", "TRY", "TTD", "TWD", "TZS",
	"UAH", "USD", "UYU", "UZS",
	"VES",
	"WST",
	"XCD",
	"YER",
	"ZAR", "ZMW", "ZWL",
}
