package workbook

import "testing"

func TestSUMIFCriteriaExactAndCaseInsensitive(t *testing.T) {
	for _, test := range []struct {
		value, criteria string
		want bool
	}{
		{"apple", "apple", true},
		{"Apple", "apple", true},
		{"apple", "APPLE", true},
		{"apple", "pear", false},
		{" apples ", "apples", false},
	} {
		got, err := matchesSUMIFCriteria(test.value, test.criteria)
		if err != nil || got != test.want {
			t.Errorf("matchesSUMIFCriteria(%q, %q) = %v, %v; want %v, nil", test.value, test.criteria, got, err, test.want)
		}
	}
}

func TestSUMIFCriteriaNumericComparisons(t *testing.T) {
	for _, test := range []struct {
		value, criteria string
		want bool
	}{
		{"5", ">4", true}, {"5", ">5", false}, {"5", ">=5", true},
		{"5", "<6", true}, {"5", "<5", false}, {"5", "<=5", true},
		{"5", "=5", true}, {"5", "<>5", false}, {"5", "<>4", true},
		{"not a number", ">4", false},
	} {
		got, err := matchesSUMIFCriteria(test.value, test.criteria)
		if err != nil || got != test.want {
			t.Errorf("matchesSUMIFCriteria(%q, %q) = %v, %v; want %v, nil", test.value, test.criteria, got, err, test.want)
		}
	}
}

func TestSUMIFCriteriaBlank(t *testing.T) {
	for _, test := range []struct {
		value, criteria string
		want bool
	}{
		{"", "", true}, {"", "=", true}, {"value", "", false}, {"value", "<>", true},
	} {
		got, err := matchesSUMIFCriteria(test.value, test.criteria)
		if err != nil || got != test.want {
			t.Errorf("matchesSUMIFCriteria(%q, %q) = %v, %v; want %v, nil", test.value, test.criteria, got, err, test.want)
		}
	}
}

func TestSUMIFCriteriaWildcardsAndEscapes(t *testing.T) {
	for _, test := range []struct {
		value, criteria string
		want bool
	}{
		{"alphabet", "alpha*", true}, {"alphabet", "a?pha*", true},
		{"alpha*", "alpha~*", true}, {"alpha?", "alpha~?", true},
		{"alpha~", "alpha~~", true}, {"alphaX", "alpha~*", false},
	} {
		got, err := matchesSUMIFCriteria(test.value, test.criteria)
		if err != nil || got != test.want {
			t.Errorf("matchesSUMIFCriteria(%q, %q) = %v, %v; want %v, nil", test.value, test.criteria, got, err, test.want)
		}
	}
	for _, criteria := range []string{">", "<", "~"} {
		if _, err := matchesSUMIFCriteria("value", criteria); err == nil {
			t.Errorf("matchesSUMIFCriteria with malformed criteria %q returned nil error", criteria)
		}
	}
}
