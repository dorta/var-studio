package boards

import (
	_ "embed"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// catalog.json is generated from the som.json files in the
// Variscite Developer
// Portal.
//
//go:embed catalog.json
var catalogJSON []byte

type Product struct {
	Name              string   `json:"name"`
	SoC               string   `json:"soc"`
	Description       string   `json:"description"`
	DocumentationURL  string   `json:"documentation_url,omitempty"`
	ShopURL           string   `json:"shop_url,omitempty"`
	SpecificationsURL string   `json:"specifications_url,omitempty"`
	QuickStartURL     string   `json:"quick_start_url,omitempty"`
	ImageURL          string   `json:"image_url,omitempty"`
	Machines          []string `json:"machines,omitempty"`
	DTBs              []string `json:"dtbs,omitempty"`
	Carriers          []string `json:"carriers,omitempty"`
	Carrier           string   `json:"carrier,omitempty"`
	Confidence        string   `json:"confidence,omitempty"`
	DetectionSource   string   `json:"detection_source,omitempty"`
}

type candidate struct {
	product Product
	score   int
	sources []string
}

func Catalog() []Product {
	var products []Product
	if err := json.Unmarshal(catalogJSON, &products); err != nil {
		return []Product{}
	}
	return products
}

func Detect(model string, compatible []string) Product {
	modelKey := normalize(model)
	compatibleKeys := make([]string, 0, len(compatible))
	for _, value := range compatible {
		compatibleKeys = append(
			compatibleKeys,
			normalize(value),
		)
	}
	var matches []candidate
	for _, product := range Catalog() {
		item := candidate{product: product}
		if containsProductName(model, product.Name) {
			item.score += 100
			item.sources = append(
				item.sources,
				"Device Tree model",
			)
		}
		for _, carrier := range product.Carriers {
			if key := normalize(carrier); key != "" &&
				strings.Contains(modelKey, key) {
				item.score += 35
				item.product.Carrier = carrier
				item.sources = append(
					item.sources,
					"carrier model",
				)
				break
			}
		}
		for _, machine := range product.Machines {
			if containsKey(
				compatibleKeys,
				normalize(machine),
			) {
				item.score += 40
				item.sources = append(
					item.sources,
					"Device Tree compatible",
				)
				break
			}
		}
		for _, dtb := range product.DTBs {
			stem := strings.TrimSuffix(
				filepath.Base(dtb),
				filepath.Ext(dtb),
			)
			if containsKey(
				compatibleKeys,
				normalize(stem),
			) ||
				strings.Contains(
					modelKey,
					normalize(stem),
				) {
				item.score += 60
				item.sources = append(
					item.sources,
					"Device Tree board identifier",
				)
				break
			}
		}
		if item.score > 0 {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return Product{
			Confidence:      "unidentified",
			DetectionSource: "No catalog rule matched the Device Tree",
		}
	}
	sort.SliceStable(
		matches,
		func(i, j int) bool { return matches[i].score > matches[j].score },
	)
	if len(matches) > 1 &&
		matches[0].score == matches[1].score {
		return Product{
			Confidence:      "ambiguous",
			DetectionSource: "Multiple products matched the Device Tree with " +
				"equal confidence",
		}
	}
	result := matches[0].product
	if matches[0].score >= 100 {
		result.Confidence = "exact"
	} else {
		result.Confidence = "compatible"
	}
	result.DetectionSource = strings.Join(
		unique(matches[0].sources),
		" + ",
	)
	return result
}

func normalize(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func containsProductName(model, name string) bool {
	canonical := func(value string) string {
		var result []rune
		separator := false
		for _, r := range strings.ToLower(value) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				if separator && len(result) > 0 {
					result = append(result, '-')
				}
				result = append(result, r)
				separator = false
			} else {
				separator = true
			}
		}
		return string(result)
	}
	haystack, needle := canonical(model), canonical(name)
	for start := 0; start < len(haystack); {
		index := strings.Index(haystack[start:], needle)
		if index < 0 {
			return false
		}
		index += start
		before := index == 0 || haystack[index-1] == '-'
		afterIndex := index + len(needle)
		after := afterIndex == len(haystack) ||
			haystack[afterIndex] == '-'
		if before && after {
			return true
		}
		start = index + 1
	}
	return false
}

func containsKey(values []string, wanted string) bool {
	if wanted == "" {
		return false
	}
	for _, value := range values {
		if strings.Contains(value, wanted) ||
			strings.Contains(wanted, value) {
			return true
		}
	}
	return false
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
