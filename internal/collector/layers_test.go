package collector

import "testing"

func TestLayerLinks(t *testing.T) {
	tests := []struct {
		name       string
		revision   string
		wantRepo   string
		wantCommit string
	}{
		{
			"meta-chromium",
			"abc123",
			"https://github.com/OSSystems/meta-browser",
				"https://github.com/OSSystems/meta-browser/commit/" + "abc123",
		},
		{
			"meta-variscite-bsp-imx",
			"def456", "https://github.com/varigit/meta-variscite-bsp-" + "imx",
				"https://github.com/varigit/meta-variscite-bsp-" + "imx/commit/def456",
		},
		{
			"meta-tpm",
			"789abc",
			"https://git.yoctoproject.org/meta-security",
				"https://git.yoctoproject.org/meta-security/" + "commit/?id=789abc",
		},
		{"unknown-layer", "123", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, commit := layerLinks(
				test.name,
				test.revision,
			)
			if repo != test.wantRepo ||
				commit != test.wantCommit {
				t.Fatalf(
					"layerLinks() = %q, %q",
					repo,
					commit,
				)
			}
		})
	}
}
