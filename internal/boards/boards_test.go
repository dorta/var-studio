package boards

import "testing"

func TestDetectsVARAndDARTWithSharedMachine(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{
			"Variscite VAR-SOM-MX8M-PLUS on Symphony-Board",
			"VAR-SOM-MX8M-PLUS",
		},
		{
			"Variscite DART-MX8M-PLUS on DT8MCustomBoard",
			"DART-MX8M-PLUS",
		},
		{
			"Variscite VAR-SOM-MX8X on Symphony-Board SD",
			"VAR-SOM-MX8X",
		},
	}
	for _, test := range tests {
		got := Detect(
			test.model,
			[]string{"variscite,imx8mp-var-dart"},
		)
		if got.Name != test.want ||
			got.Confidence != "exact" {
			t.Fatalf(
				"Detect(%q) = %q (%s), want %q exact",
				test.model,
				got.Name,
				got.Confidence,
				test.want,
			)
		}
	}
}

func TestAmbiguousMachineDoesNotGuess(t *testing.T) {
	got := Detect(
		"Generic i.MX8M Plus board",
		[]string{"variscite,imx8mp-var-dart"},
	)
	if got.Confidence != "ambiguous" {
		t.Fatalf(
			"confidence = %q, want ambiguous (product %q)",
			got.Confidence,
			got.Name,
		)
	}
}

func TestCatalogIncludesAllPortalProducts(t *testing.T) {
	if got := len(Catalog()); got != 33 {
		t.Fatalf(
			"catalog contains %d products, want 33",
			got,
		)
	}
}
