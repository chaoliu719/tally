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
//
// FormatMajor and ParseMajor convert between that smallest-unit integer (how
// tally stores amounts internally) and the decimal string, in code's major
// unit, that crosses the MCP tool boundary (see FormatMajor/ParseMajor docs
// and the unify-transaction-amount-format change). Both do the conversion
// with string/integer arithmetic only -- never float64 or math/big.Float --
// so it is always exact.
package currency

import (
	"fmt"
	"strconv"
	"strings"
)

// Decimals reports the number of digits after the decimal point in code's
// smallest unit (e.g. 2 for "USD", 0 for "JPY", 3 for "BHD"), and whether
// code is a supported ISO 4217 currency code at all.
func Decimals(code string) (digits int, ok bool) {
	digits, ok = decimalDigits[code]
	return digits, ok
}

// FormatMajor renders minorUnits -- an integer count of code's smallest unit
// (e.g. fen for CNY, the yen itself for JPY, fils for BHD) -- as a decimal
// string in code's major unit, with exactly code's standard number of
// fractional digits (e.g. 5000 minor units renders as "50.00" for CNY,
// "5000" for JPY, "5.000" for BHD). It returns an error if code is not a
// supported currency code.
func FormatMajor(code string, minorUnits int64) (string, error) {
	digits, ok := Decimals(code)
	if !ok {
		return "", fmt.Errorf("unsupported currency: %q", code)
	}

	neg := minorUnits < 0
	abs := minorUnits
	if neg {
		abs = -abs
	}

	s := strconv.FormatInt(abs, 10)
	if digits == 0 {
		if neg {
			s = "-" + s
		}
		return s, nil
	}

	if len(s) <= digits {
		s = strings.Repeat("0", digits-len(s)+1) + s
	}
	whole, frac := s[:len(s)-digits], s[len(s)-digits:]

	result := whole + "." + frac
	if neg {
		result = "-" + result
	}
	return result, nil
}

// ParseMajor parses s, a decimal string in code's major unit (e.g. "50.00"
// for CNY, "5000" for JPY, "5.000" for BHD), into an integer count of code's
// smallest unit. It rejects: an unsupported currency code; a string that
// isn't a plain decimal number (non-digit characters, more than one decimal
// point, an empty string); a string with more fractional digits than code's
// standard precision allows; and a value that doesn't parse to a positive
// number (amount is always positive on the wire -- the sign of income vs.
// expense is carried by the transaction's type, not by this string).
func ParseMajor(code string, s string) (minorUnits int64, err error) {
	digits, ok := Decimals(code)
	if !ok {
		return 0, fmt.Errorf("unsupported currency: %q", code)
	}

	orig := s
	neg := false
	switch {
	case strings.HasPrefix(s, "-"):
		neg = true
		s = s[1:]
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	}

	whole, frac, hasFrac := strings.Cut(s, ".")
	if strings.Contains(frac, ".") {
		return 0, fmt.Errorf("invalid amount %q: multiple decimal points", orig)
	}
	if whole == "" || !isDigits(whole) {
		return 0, fmt.Errorf("invalid amount %q: not a valid decimal number", orig)
	}
	if hasFrac {
		if frac == "" || !isDigits(frac) {
			return 0, fmt.Errorf("invalid amount %q: not a valid decimal number", orig)
		}
		if len(frac) > digits {
			return 0, fmt.Errorf("invalid amount %q: %s allows at most %d decimal digit(s)", orig, code, digits)
		}
	}
	frac += strings.Repeat("0", digits-len(frac))

	value, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", orig, err)
	}
	if neg {
		value = -value
	}
	if value <= 0 {
		return 0, fmt.Errorf("invalid amount %q: must be positive", orig)
	}
	return value, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
