// internal/domain/dedupe.go
package domain

import (
	"sort"
	"strings"

	"aethonx/internal/core"
)

// DedupeAndNormalize normaliza artifacts y elimina duplicados.
// Si un mismo artifact aparece en varias fuentes, combina sus "sources" en Meta["sources"].
func DedupeAndNormalize(in []core.Artifact) []core.Artifact {
	type k struct{ typ, val string }

	seen := make(map[k]int) // key -> index en out
	out := make([]core.Artifact, 0, len(in))

	for _, a := range in {
		a = normalizeArtifact(a)
		key := k{typ: a.Type, val: a.Value}

		if idx, ok := seen[key]; ok {
			// Merge de meta y fuentes
			ex := out[idx]
			ex.Meta = mergeMetaSources(ex, a)
			out[idx] = ex
			continue
		}
		seen[key] = len(out)
		out = append(out, a)
	}

	// Orden estable: por Type luego Value (útil para output/tablas)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type == out[j].Type {
			return out[i].Value < out[j].Value
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func normalizeArtifact(a core.Artifact) core.Artifact {
	if a.Meta == nil {
		a.Meta = make(map[string]string)
	}
	switch a.Type {
	case "domain", "subdomain":
		a.Value = normalizeDomain(a.Value)
	case "email":
		a.Value = normalizeEmail(a.Value)
	default:
		a.Value = strings.TrimSpace(a.Value)
	}
	// inicializa sources
	if a.Source != "" {
		a.Meta["sources"] = addToCSV(a.Meta["sources"], a.Source)
	}
	return a
}

func normalizeDomain(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, ".")
	v = strings.TrimPrefix(v, "*.")
	v = strings.ToLower(v)
	return v
}

func normalizeEmail(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ToLower(v)
	return v
}

func mergeMetaSources(a, b core.Artifact) map[string]string {
	out := make(map[string]string, len(a.Meta)+len(b.Meta)+1)
	for k, v := range a.Meta {
		out[k] = v
	}
	for k, v := range b.Meta {
		if k == "sources" {
			continue
		}
		// no pisar claves existentes salvo que estén vacías
		if _, ok := out[k]; !ok || out[k] == "" {
			out[k] = v
		}
	}
	out["sources"] = mergeCSV(a.Meta["sources"], b.Meta["sources"])
	return out
}

func addToCSV(csv, item string) string {
	if item == "" {
		return csv
	}
	if csv == "" {
		return item
	}
	// evita duplicados simples
	for _, s := range strings.Split(csv, ",") {
		if strings.TrimSpace(s) == item {
			return csv
		}
	}
	return csv + "," + item
}

func mergeCSV(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	seen := map[string]bool{}
	out := []string{}
	for _, s := range strings.Split(a+","+b, ",") {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return strings.Join(out, ",")
}
