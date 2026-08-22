package currency_test

import (
	"testing"

	"tally/internal/currency"
)

func TestCommonCurrenciesHaveTwoDecimals(t *testing.T) {
	for _, code := range []string{"CNY", "USD", "EUR"} {
		digits, ok := currency.Decimals(code)
		if !ok {
			t.Fatalf("%s: expected to be supported", code)
		}
		if digits != 2 {
			t.Errorf("%s: digits = %d, want 2", code, digits)
		}
	}
}

func TestZeroDecimalCurrencies(t *testing.T) {
	for _, code := range []string{"JPY", "KRW"} {
		digits, ok := currency.Decimals(code)
		if !ok {
			t.Fatalf("%s: expected to be supported", code)
		}
		if digits != 0 {
			t.Errorf("%s: digits = %d, want 0", code, digits)
		}
	}
}

func TestThreeDecimalCurrencies(t *testing.T) {
	for _, code := range []string{"BHD", "KWD", "OMR"} {
		digits, ok := currency.Decimals(code)
		if !ok {
			t.Fatalf("%s: expected to be supported", code)
		}
		if digits != 3 {
			t.Errorf("%s: digits = %d, want 3", code, digits)
		}
	}
}

func TestUnsupportedCurrency(t *testing.T) {
	if currency.Supported("NOTACURRENCY") {
		t.Fatal("expected NOTACURRENCY to be unsupported")
	}
	if _, ok := currency.Decimals("NOTACURRENCY"); ok {
		t.Fatal("expected NOTACURRENCY to be unsupported")
	}
}
