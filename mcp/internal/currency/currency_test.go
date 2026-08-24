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

func TestFormatMajor(t *testing.T) {
	cases := []struct {
		code       string
		minorUnits int64
		want       string
	}{
		{"CNY", 5000, "50.00"},
		{"CNY", 1, "0.01"},
		{"CNY", 0, "0.00"},
		{"JPY", 5000, "5000"},
		{"JPY", 0, "0"},
		{"BHD", 5000, "5.000"},
		{"BHD", 1, "0.001"},
	}
	for _, c := range cases {
		got, err := currency.FormatMajor(c.code, c.minorUnits)
		if err != nil {
			t.Errorf("FormatMajor(%q, %d): unexpected error: %v", c.code, c.minorUnits, err)
			continue
		}
		if got != c.want {
			t.Errorf("FormatMajor(%q, %d) = %q, want %q", c.code, c.minorUnits, got, c.want)
		}
	}
}

func TestFormatMajorUnsupportedCurrency(t *testing.T) {
	if _, err := currency.FormatMajor("NOTACURRENCY", 100); err == nil {
		t.Fatal("expected an error for an unsupported currency")
	}
}

func TestParseMajor(t *testing.T) {
	cases := []struct {
		code string
		s    string
		want int64
	}{
		{"CNY", "50.00", 5000},
		{"CNY", "50", 5000},
		{"CNY", "0.01", 1},
		{"JPY", "5000", 5000},
		{"BHD", "5.000", 5000},
		{"BHD", "5", 5000},
		{"BHD", "0.001", 1},
	}
	for _, c := range cases {
		got, err := currency.ParseMajor(c.code, c.s)
		if err != nil {
			t.Errorf("ParseMajor(%q, %q): unexpected error: %v", c.code, c.s, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseMajor(%q, %q) = %d, want %d", c.code, c.s, got, c.want)
		}
	}
}

func TestParseMajorRoundTrip(t *testing.T) {
	for _, code := range []string{"CNY", "JPY", "BHD"} {
		for _, minorUnits := range []int64{1, 50, 5000, 123456} {
			s, err := currency.FormatMajor(code, minorUnits)
			if err != nil {
				t.Fatalf("FormatMajor(%q, %d): %v", code, minorUnits, err)
			}
			got, err := currency.ParseMajor(code, s)
			if err != nil {
				t.Fatalf("ParseMajor(%q, %q): %v", code, s, err)
			}
			if got != minorUnits {
				t.Errorf("round trip %q/%d: FormatMajor -> %q -> ParseMajor -> %d", code, minorUnits, s, got)
			}
		}
	}
}

func TestParseMajorInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		code string
		s    string
	}{
		{"non-numeric characters", "CNY", "50.0a"},
		{"multiple decimal points", "CNY", "5.0.0"},
		{"empty string", "CNY", ""},
		{"precision exceeds CNY", "CNY", "50.001"},
		{"precision exceeds JPY", "JPY", "50.5"},
		{"precision exceeds BHD", "BHD", "5.0001"},
		{"zero", "CNY", "0.00"},
		{"negative", "CNY", "-10.00"},
		{"unsupported currency", "NOTACURRENCY", "50.00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := currency.ParseMajor(c.code, c.s); err == nil {
				t.Fatalf("ParseMajor(%q, %q): expected an error, got none", c.code, c.s)
			}
		})
	}
}
